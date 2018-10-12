package main

import (
	"flag"
	"fmt"
	. "github.com/SubutaiBogatur/Peerster/models"
	. "github.com/SubutaiBogatur/Peerster/utils"
	"github.com/dedis/protobuf"
	log "github.com/sirupsen/logrus"
	"math/rand"
	. "net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Some documentation on what's happening
//
// We will have threads:
// * client-reader      thread : the only port reading from client socket. Sends client message to message-processor
// * peer-reader        thread : the only port reading from peer socket. Sends GossipPackets to message-processor
// * peer-writer        thread : the only port writing to peer socket. Listens to channel for GossipPackets and writes them
// * anti-entropy-timer thread : goroutine sends a status to random peer every timeout seconds
// * message-processor  thread : is an abstraction on client-listener, peer-listener threads. Receives all the messages and for every message:
//     + rumor-msg  : upd status, send status back, send rumor further, start rumor-mongering thread waiting for status
//     + status-msg : push it to one of the rumor-mongering threads (if rumor mongering is not in progress, then compare statuses and start it)
// * rumor-mongering    thread : thread waits either for status-msg to arrive or for timeout and stores rumor-msg, it was initiated for
//     + status-msg : cmp (store sync-safe Map for VectorClock, which are edited from message-processor) and send new msg via peer-communicator
//     + timeout    : 1/2 & send new rumor-msg via peer-communicator

// command line arguments
var (
	UIPort     = flag.Int("UIPort", 4848, "Port, where gossiper listens for client. Client is situated on the same machine, so gossiper listens to "+LocalIp+":port for client")
	gossipAddr = flag.String("gossipAddr", "127.0.0.1:1212", "Address, where gossiper is launched: ip:port. Other peers will contact gossiper through this peersAddress")
	name       = flag.String("name", "go_rbachev", "Gossiper name")
	peers      = flag.String("peers", "", "Other gossipers' addresses separated with \",\" in the form ip:port")
	simpleMode = flag.Bool("simple", false, "True, if mode is simple")

	logger = log.WithField("bin", "gos")

	clientMessagesToProcess = make(chan *ClientMessage)
	peerMessagesToProcess   = make(chan *AddressedGossipPacket)
	peerMessagesToSend      = make(chan *AddressedGossipPacket)

	// map is accessed from message-processor and from rumor-mongering threads, should be converted to sync.Map{}
	statusesChannels   = make(map[string]chan *StatusPacket) // peerIp -> channel, where rumor-mongering goroutine is waiting for status feedback
	statusesChannelMux sync.Mutex
)

const (
	RumorTimeout       = time.Second
	AntiEntropyTimeout = time.Second
)

func initGossiper() (Gossiper, error) {
	flag.Parse()

	g := Gossiper{messageStorage: &MessageStorage{VectorClock: make(map[string]uint32), Messages: make(map[string][]*RumorMessage)}}

	clientListenAddress := LocalIp + ":" + strconv.Itoa(*UIPort)
	clientListenUdpAddress, err := ResolveUDPAddr("udp4", clientListenAddress)
	if err != nil {
		logger.Error("Unable to parse clientListenAddress: " + string(clientListenAddress))
		return g, err
	}
	g.clientAddress = clientListenUdpAddress

	peersListenAddress := *gossipAddr
	peersListenUdpAddress, err := ResolveUDPAddr("udp4", peersListenAddress)
	if err != nil {
		logger.Error("Unable to parse peersListenAddress: " + string(peersListenAddress))
		return g, err
	}
	g.peersAddress = peersListenUdpAddress
	logger = logger.WithField("a", peersListenUdpAddress.String())

	gossiperName := *name
	if gossiperName == "" {
		return g, PeersterError{ErrorMsg: "Empty name provided"}
	}
	g.name = gossiperName

	peersArr := strings.Split(*peers, ",")
	for i := 0; i < len(peersArr); i++ {
		if peersArr[i] == "" {
			continue
		}

		peerAddr, err := ResolveUDPAddr("udp4", peersArr[i])
		if err != nil {
			logger.Error("Error when parsing peers")
			return g, err
		}
		g.peers = append(g.peers, peerAddr)
	}

	// command line arguments parsed, start listening:
	peersUdpConn, err := ListenUDP("udp4", g.peersAddress)
	if err != nil {
		return g, err
	}
	g.peersConnection = peersUdpConn

	clientUdpConn, err := ListenUDP("udp4", g.clientAddress)
	if err != nil {
		return g, err
	}
	g.clientConnection = clientUdpConn

	return g, nil
}

type Gossiper struct {
	peersAddress    *UDPAddr // peersAddress for peers
	peersConnection *UDPConn

	clientAddress    *UDPAddr
	clientConnection *UDPConn

	peers         []*UDPAddr // accessed from message-processor and from rumor-mongering
	peersSliceMux sync.Mutex

	messageStorage *MessageStorage // accessed from message-processor and from rumor-mongering, is hard-synchronized

	name string
}

// client-reader thread
func (g *Gossiper) startClientReader() {
	logger.Info("starting reading bytes on client: " + g.clientAddress.String())
	var buffer = make([]byte, MaxPacketSize)

	for {
		g.clientConnection.ReadFromUDP(buffer)

		cmsg := &ClientMessage{}
		if err := protobuf.Decode(buffer, cmsg); err != nil {
			logger.Warn("unable to decode message, error: " + err.Error())
		}

		// ~~~ put into channel ~~~
		logger.Debug("put client message into channel")
		clientMessagesToProcess <- cmsg
		// ~~~~~~~~~~~~~~~~~~~~~~~~
	}
}

// peer-reader thread
func (g *Gossiper) startPeerReader() {
	logger.Info("starting reading bytes on peer: " + g.peersAddress.String())
	var buffer = make([]byte, MaxPacketSize)

	for {
		_, addr, _ := g.peersConnection.ReadFromUDP(buffer)

		gp := &GossipPacket{}
		if err := protobuf.Decode(buffer, gp); err != nil {
			logger.Warn("unable to decode message, error: " + err.Error())
		}

		apg := &AddressedGossipPacket{Address: addr, Packet: gp}

		// ~~~ put into channel ~~~
		logger.Debug("put peer message into channel")
		peerMessagesToProcess <- apg
		// ~~~~~~~~~~~~~~~~~~~~~~~~
	}
}

// peer-writer thread
func (g *Gossiper) startPeerWriter() {
	logger.Info("starting peer writer thread")

	for {
		// ~~~ read from channel ~~~
		agp := <-peerMessagesToSend
		// ~~~~~~~~~~~~~~~~~~~~~~~~~

		address := agp.Address
		gp := agp.Packet

		packetBytes, err := protobuf.Encode(gp)
		if err != nil {
			logger.Error("unable to encode msg: " + err.Error())
			continue
		}

		logger.Debug("sending message from " + g.peersConnection.LocalAddr().String() + " to " + address.String())

		n, err := g.peersConnection.WriteToUDP(packetBytes, address)
		if err != nil {
			logger.Error("error when writing to connection: " + err.Error() + " n is " + strconv.Itoa(n))
			continue
		}
	}
}

func (g *Gossiper) startAntiEntropyTimer() {
	logger.Info("starting anti-entropy timer thread")

	for {
		ticker := time.NewTicker(AntiEntropyTimeout)
		<-ticker.C // wait for the timer to shoot

		if g.emptyPeers() {
			<-time.NewTicker(AntiEntropyTimeout * 5).C // wait additional time
			continue
		}

		// send status to a random peer
		peer := g.getRandomPeer()
		peerMessagesToSend <- &AddressedGossipPacket{Address: peer, Packet: &GossipPacket{Status: g.messageStorage.GetCurrentStatusPacket()}}
	}
}

func (g *Gossiper) getRandomPeer() *UDPAddr {
	g.peersSliceMux.Lock()
	defer g.peersSliceMux.Unlock()

	randomInt := rand.Int31n(int32(len(g.peers))) // end not inclusive
	randomPeer := g.peers[randomInt]

	return randomPeer
}

func (g *Gossiper) emptyPeers() bool {
	g.peersSliceMux.Lock()
	defer g.peersSliceMux.Unlock()

	return len(g.peers) == 0
}

func (g *Gossiper) getPeersCopy() []*UDPAddr {
	g.peersSliceMux.Lock()
	defer g.peersSliceMux.Unlock()

	copySlice := make([]*UDPAddr, len(g.peers))
	copy(copySlice, g.peers)

	return copySlice
}

func (g *Gossiper) printPeers() {
	g.peersSliceMux.Lock()
	defer g.peersSliceMux.Unlock()

	fmt.Print("PEERS ")
	for i := 0; i < len(g.peers); i++ {
		if i == len(g.peers)-1 {
			fmt.Print(g.peers[i].String())
		} else {
			fmt.Print(g.peers[i].String() + ",")
		}
	}
	fmt.Println("")
}

// --------------------------------------
// message-processor thread section:
// --------------------------------------

func (g *Gossiper) startMessageProcessor() {
	logger.Info("starting message processor")

	for {
		select {
		case cmsg := <-clientMessagesToProcess:
			logger.Debug("got client message from channel: " + cmsg.Text)
			cmsg.Print()
			g.printPeers()
			g.processClientMessage(cmsg)
		case agp := <-peerMessagesToProcess:
			logger.Debug("got peer message from channel")
			agp.Print()
			g.printPeers()
			g.processAddressedGossipPacket(agp)
		}
	}
}

func (g *Gossiper) processClientMessage(cmsg *ClientMessage) {
	logger.Info("got client message: " + cmsg.Text)
	if !*simpleMode {
		messageId := g.messageStorage.GetNextMessageId(g.name)
		rmsg := &RumorMessage{OriginalName: g.name, ID: messageId, Text: cmsg.Text}
		g.processRumorMessage(rmsg)
	} else {
		logger.Info("mode is simple, so client message is distributed as simple one")
		smsg := &SimpleMessage{Text:cmsg.Text, OriginalName:g.name}
		g.processAddressedSimpleMessage(smsg, nil)
	}
}

func (g *Gossiper) processAddressedGossipPacket(agp *AddressedGossipPacket) {
	gp := agp.Packet
	address := agp.Address

	g.updatePeersIfNeeded(address)

	if gp.Rumor != nil {
		logger.Info("got rumor-msg " + gp.Rumor.String() + " from " + address.String())
		g.processAddressedRumorMessage(gp.Rumor, address)
	} else if gp.Status != nil {
		logger.Info("got status from " + address.String())
		g.processAddressedStatusPacket(gp.Status, address)
	} else if gp.Simple != nil {
		logger.Info("got simple from " + address.String())
		g.processAddressedSimpleMessage(gp.Simple, address)
	}
}

func (g *Gossiper) processAddressedSimpleMessage(smsg *SimpleMessage, address *UDPAddr) {
	g.peersSliceMux.Lock()
	defer g.peersSliceMux.Unlock()

	smsg.RelayPeerAddr = g.peersConnection.LocalAddr().String()

	for _, peer := range g.peers {
		if address != nil && peer.String() == address.String() {
			continue
		}
		logger.Info("sending simple to " + peer.String())
		peerMessagesToSend <- &AddressedGossipPacket{Address: peer, Packet: &GossipPacket{Simple: smsg}}
	}
}

func (g *Gossiper) processAddressedStatusPacket(sp *StatusPacket, address *UDPAddr) {
	statusesChannelMux.Lock()
	if val, isPresent := statusesChannels[address.String()]; isPresent {
		logger.Info("status from " + address.String() + " found in map, forwarding it to corresponding goroutine...")
		val <- sp
		delete(statusesChannels, address.String()) // goroutine cannot eat more, than one status packet, so delete it from map
		statusesChannelMux.Unlock()
		return
	}
	statusesChannelMux.Unlock()

	logger.Info("got status not from map, interesting")
	rmsg, otherHasSomethingNew := g.messageStorage.Diff(sp)
	if rmsg != nil {
		g.spreadTheRumor(rmsg, address)
	} else if otherHasSomethingNew {
		peerMessagesToSend <- &AddressedGossipPacket{Packet: &GossipPacket{Status: g.messageStorage.GetCurrentStatusPacket()}, Address: address}
	} else {
		log.Info("nothing interesting in the status")
		fmt.Println("IN SYNC WITH " + address.String())
	}
	// else do nothing at all
}

func (g *Gossiper) processAddressedRumorMessage(rmsg *RumorMessage, address *UDPAddr) {

	// send status back to rumorer:
	feedbackStatus := &GossipPacket{Status: g.messageStorage.GetCurrentStatusPacket()}
	addressedFeedbackStatus := &AddressedGossipPacket{Address: address, Packet: feedbackStatus}
	logger.Info("sending status as feedback to " + address.String())
	peerMessagesToSend <- addressedFeedbackStatus

	g.processRumorMessage(rmsg)
}

func (g *Gossiper) updatePeersIfNeeded(peer *UDPAddr) {
	g.peersSliceMux.Lock()
	defer g.peersSliceMux.Unlock()

	for i := 0; i < len(g.peers); i++ {
		if g.peers[i].String() == peer.String() {
			return // peer is a known one
		}
	}
	g.peers = append(g.peers, peer)
}

func (g *Gossiper) processRumorMessage(rmsg *RumorMessage) {
	isNewMessage := g.messageStorage.AddRumorMessage(rmsg)

	if isNewMessage {
		// rumormongering -- choose random peer to send rmsg to
		if g.emptyPeers() {
			logger.Warn("no peers are known, cannot do rumor-mongering")
			return // msg came from client and no peers are known
		}

		g.spreadTheRumor(rmsg, nil)
	} else {
		// if we got an old rumor, then let's do nothing, we already have sent a feedback, everything is good
		logger.Info("message is not new, skipping it")
	}

}

// -------------------------------------------
// end of message-processor thread section
// -------------------------------------------

// called both by message-processor and rumor-mongering threads
// if peer is nil, choose random peer
func (g *Gossiper) spreadTheRumor(rmsg *RumorMessage, peer *UDPAddr) {

	if peer == nil {
		peer = g.getRandomPeer()
	}

	statusesChannelMux.Lock()
	if _, contains := statusesChannels[peer.String()]; contains {
		logger.Warn("rumormongering with this peer is already in progress, this case is too hard for me")
		statusesChannelMux.Unlock()
		return
	}
	ch := make(chan *StatusPacket)
	statusesChannels[peer.String()] = ch
	statusesChannelMux.Unlock()

	logger.Info("rumor sent further to " + peer.String())
	peerMessagesToSend <- &AddressedGossipPacket{Address: peer, Packet: &GossipPacket{Rumor: rmsg}}
	fmt.Println("MONGERING WITH " + peer.String())

	go g.startRumorMongeringThread(rmsg, ch, peer)
}

// --------------------------------------
// rumor-mongering thread section:
// --------------------------------------

func (g *Gossiper) startRumorMongeringThread(messageBeingRumored *RumorMessage, ch chan *StatusPacket, peer *UDPAddr) {
	ticker := time.NewTicker(RumorTimeout)
	select {
	case <-ticker.C:
		logger.Info("peer " + peer.String() + " exceeded the timeout")

		// clean the map
		statusesChannelMux.Lock()
		delete(statusesChannels, peer.String())
		statusesChannelMux.Unlock()

		g.flipRumorMongeringCoin(messageBeingRumored)
	case statusPacket := <-ch:
		logger.Info("processing status-response in rumor-mongering thread from " + peer.String())

		rmsg, otherHasSomethingNew := g.messageStorage.Diff(statusPacket)
		if rmsg != nil {
			logger.Info("peer " + peer.String() + " doesn't know rmsg, sending it: " + rmsg.String())
			g.spreadTheRumor(rmsg, peer)
		} else if otherHasSomethingNew {
			logger.Info("peer " + peer.String() + " knows more, than me, sending status to him")
			agp := &AddressedGossipPacket{Packet: &GossipPacket{Status: g.messageStorage.GetCurrentStatusPacket()}, Address: peer}
			peerMessagesToSend <- agp
		} else {
			fmt.Println("IN SYNC WITH " + peer.String())
			logger.Info("peer " + peer.String() + " has same info as me, flipping the coin")
			g.flipRumorMongeringCoin(messageBeingRumored)
		}
	}
	// let the goroutine die!
}

func (g *Gossiper) flipRumorMongeringCoin(messageBeingRumored *RumorMessage) {
	continueMongering := (rand.Int() % 2) == 0
	if continueMongering {
		peer := g.getRandomPeer() // crutch because of fmt. requirements in HW1 :(
		logger.Info("coin says to continue rumor-mongering")
		fmt.Println("FLIPPED COIN sending rumor to " + peer.String())
		g.spreadTheRumor(messageBeingRumored, peer)
	} else {
		logger.Info("coin says to stop rumor-mongering of " + messageBeingRumored.String())
	}
}

// --------------------------------------
// end of rumor-mongering thread section
// --------------------------------------

func main() {
	log.SetLevel(log.FatalLevel)

	customFormatter := new(log.TextFormatter)
	//customFormatter.TimestampFormat = "15:04:05:05"
	//customFormatter.FullTimestamp = true
	log.SetFormatter(customFormatter)

	g, err := initGossiper()
	if err != nil {
		logger.Fatal("gossiper failed to construct itself with error: " + err.Error())
		return
	}

	// set random seed
	rand.Seed(time.Now().Unix())

	go g.startClientReader()
	go g.startPeerReader()
	go g.startPeerWriter()
	go g.startAntiEntropyTimer()

	g.startMessageProcessor() // goroutine dies, when app dies, so blocking function is called in main thread

	fmt.Println("gossiper finished")
}
