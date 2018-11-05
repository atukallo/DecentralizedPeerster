package gossiper

import (
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
	"sync/atomic"
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

type Gossiper struct {
	name atomic.Value // name can be changed by webserver and is accessed from message-processor, thus using atomic string

	peersAddress    *UDPAddr // peersAddress for peers
	peersConnection *UDPConn

	clientAddress    *UDPAddr
	clientConnection *UDPConn

	peers         []*UDPAddr // accessed from message-processor and from rumor-mongering
	peersSliceMux sync.Mutex

	messageStorage *MessageStorage // accessed from message-processor and from rumor-mongering, is hard-synchronized

	isSimpleMode bool       // in simple mode sending only simple messages
	l            *log.Entry // logger
}

func NewGossiper(name string, uiport int, peersAddress string, peers string, isSimpleMode bool) (*Gossiper, error) {
	logger := log.WithField("bin", "gos")

	g := &Gossiper{messageStorage: &MessageStorage{VectorClock: make(map[string]uint32), Messages: make(map[string][]*RumorMessage)}}
	g.l = logger
	g.isSimpleMode = isSimpleMode

	clientListenAddress := LocalIp + ":" + strconv.Itoa(uiport)
	clientListenUdpAddress, err := ResolveUDPAddr("udp4", clientListenAddress)
	if err != nil {
		g.l.Error("Unable to parse clientListenAddress: " + string(clientListenAddress))
		return g, err
	}
	g.clientAddress = clientListenUdpAddress

	peersListenUdpAddress, err := ResolveUDPAddr("udp4", peersAddress)
	if err != nil {
		g.l.Error("Unable to parse peersListenAddress: " + string(peersAddress))
		return g, err
	}
	g.peersAddress = peersListenUdpAddress
	g.l = g.l.WithField("a", peersListenUdpAddress.String())

	if name == "" {
		return g, PeersterError{ErrorMsg: "Empty name provided"}
	}
	g.name.Store(name)

	peersArr := strings.Split(peers, ",")
	for i := 0; i < len(peersArr); i++ {
		if peersArr[i] == "" {
			continue
		}

		peerAddr, err := ResolveUDPAddr("udp4", peersArr[i])
		if err != nil {
			g.l.Error("Error when parsing peers")
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

//helper functions:
func (g *Gossiper) getRandomPeer() *UDPAddr {
	g.peersSliceMux.Lock()
	defer g.peersSliceMux.Unlock()

	randomInt := rand.Int31n(int32(len(g.peers))) // end not inclusive
	randomPeer := g.peers[randomInt]

	return randomPeer
}

func (g *Gossiper) arePeersEmpty() bool {
	g.peersSliceMux.Lock()
	defer g.peersSliceMux.Unlock()

	return len(g.peers) == 0
}

func (g *Gossiper) GetPeersCopy() []*UDPAddr {
	g.peersSliceMux.Lock()
	defer g.peersSliceMux.Unlock()

	copySlice := make([]*UDPAddr, len(g.peers))
	copy(copySlice, g.peers)

	return copySlice
}

func (g *Gossiper) UpdatePeersIfNeeded(peer *UDPAddr) {
	g.peersSliceMux.Lock()
	defer g.peersSliceMux.Unlock()

	for i := 0; i < len(g.peers); i++ {
		if g.peers[i].String() == peer.String() {
			return // peer is a known one
		}
	}
	g.peers = append(g.peers, peer)
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

func (g *Gossiper) GetName() string {
	return g.name.Load().(string)
}

func (g *Gossiper) SetName(name string) {
	g.name.Store(name)
}

func (g *Gossiper) GetID(name string) uint32 {
	return g.messageStorage.GetNextMessageId(name)
}

func (g *Gossiper) GetPeerAddress() *UDPAddr {
	return g.peersAddress
}

func (g *Gossiper) GetClientAddress() *UDPAddr {
	return g.clientAddress
}

func (g *Gossiper) GetMessages() *[]RumorMessage {
	return g.messageStorage.GetMessagesCopy()
}

// client-reader thread
func (g *Gossiper) StartClientReader() {
	g.l.Info("starting reading bytes on client: " + g.clientAddress.String() + " thread")
	var buffer = make([]byte, MaxPacketSize)

	for {
		g.clientConnection.ReadFromUDP(buffer)

		cmsg := &ClientMessage{}
		if err := protobuf.Decode(buffer, cmsg); err != nil {
			// todo(atukallo): fix some protobuf warnings
			//g.l.Warn("unable to decode message, error: " + err.Error())
		}

		// ~~~ put into channel ~~~
		g.l.Debug("put client message into channel")
		clientMessagesToProcess <- cmsg
		// ~~~~~~~~~~~~~~~~~~~~~~~~
	}
}

// peer-reader thread
func (g *Gossiper) StartPeerReader() {
	g.l.Info("gossiper " + g.name.Load().(string) + ": starting reading bytes on peer: " + g.peersAddress.String() + " in new thread")
	var buffer = make([]byte, MaxPacketSize)

	for {
		_, addr, _ := g.peersConnection.ReadFromUDP(buffer)

		gp := &GossipPacket{}
		if err := protobuf.Decode(buffer, gp); err != nil {
			// todo(atukallo): fix some protobuf warnings
			//g.l.Warn("unable to decode message, error: " + err.Error())
		}

		apg := &AddressedGossipPacket{Address: addr, Packet: gp}

		// ~~~ put into channel ~~~
		g.l.Debug("put peer message into channel")
		peerMessagesToProcess <- apg
		// ~~~~~~~~~~~~~~~~~~~~~~~~
	}
}

// peer-writer thread
func (g *Gossiper) StartPeerWriter() {
	g.l.Info("starting peer writer thread")

	for {
		// ~~~ read from channel ~~~
		agp := <-peerMessagesToSend
		// ~~~~~~~~~~~~~~~~~~~~~~~~~

		address := agp.Address
		gp := agp.Packet

		packetBytes, err := protobuf.Encode(gp)
		if err != nil {
			g.l.Error("unable to encode msg: " + err.Error())
			continue
		}

		g.l.Debug("sending message from " + g.peersConnection.LocalAddr().String() + " to " + address.String())

		n, err := g.peersConnection.WriteToUDP(packetBytes, address)
		if err != nil {
			g.l.Error("error when writing to connection: " + err.Error() + " n is " + strconv.Itoa(n))
			continue
		}
	}
}

// anti-entropy thread
func (g *Gossiper) StartAntiEntropyTimer() {
	g.l.Info("starting anti-entropy timer thread")

	for {
		ticker := time.NewTicker(AntiEntropyTimeout)
		<-ticker.C // wait for the timer to shoot

		if g.arePeersEmpty() {
			<-time.NewTicker(AntiEntropyTimeout * 5).C // wait additional time
			continue
		}

		// send status to a random peer
		peer := g.getRandomPeer()
		peerMessagesToSend <- &AddressedGossipPacket{Address: peer, Packet: &GossipPacket{Status: g.messageStorage.GetCurrentStatusPacket()}}
	}
}

// --------------------------------------
// message-processor thread section:
// --------------------------------------

func (g *Gossiper) StartMessageProcessor() {
	g.l.Info("starting message processor")

	for {
		select {
		case cmsg := <-clientMessagesToProcess:
			g.l.Debug("got client message from channel: " + cmsg.Text)
			cmsg.Print()
			g.printPeers()
			g.processClientMessage(cmsg)
		case agp := <-peerMessagesToProcess:
			g.l.Debug("got peer message from channel")
			agp.Print()
			g.printPeers()
			g.processAddressedGossipPacket(agp)
		}
	}
}

func (g *Gossiper) processClientMessage(cmsg *ClientMessage) {
	g.l.Info("got client message: " + cmsg.Text)
	gossiperName := g.name.Load().(string)
	if !g.isSimpleMode {
		messageId := g.messageStorage.GetNextMessageId(gossiperName)
		rmsg := &RumorMessage{OriginalName: gossiperName, ID: messageId, Text: cmsg.Text}
		g.processRumorMessage(rmsg)
	} else {
		g.l.Info("mode is simple, so client message is distributed as simple one")
		smsg := &SimpleMessage{Text: cmsg.Text, OriginalName: gossiperName}
		g.processAddressedSimpleMessage(smsg, nil)
	}
}

func (g *Gossiper) processAddressedGossipPacket(agp *AddressedGossipPacket) {
	gp := agp.Packet
	address := agp.Address

	g.UpdatePeersIfNeeded(address)

	if gp.Rumor != nil {
		g.l.Info("got rumor-msg " + gp.Rumor.String() + " from " + address.String())
		g.processAddressedRumorMessage(gp.Rumor, address)
	} else if gp.Status != nil {
		g.l.Info("got status from " + address.String())
		g.processAddressedStatusPacket(gp.Status, address)
	} else if gp.Simple != nil {
		g.l.Info("got simple from " + address.String())
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
		g.l.Info("sending simple to " + peer.String())
		peerMessagesToSend <- &AddressedGossipPacket{Address: peer, Packet: &GossipPacket{Simple: smsg}}
	}
}

func (g *Gossiper) processAddressedStatusPacket(sp *StatusPacket, address *UDPAddr) {
	statusesChannelMux.Lock()
	if val, isPresent := statusesChannels[address.String()]; isPresent {
		g.l.Info("status from " + address.String() + " found in map, forwarding it to corresponding goroutine...")
		val <- sp
		delete(statusesChannels, address.String()) // goroutine cannot eat more, than one status packet, so delete it from map
		statusesChannelMux.Unlock()
		return
	}
	statusesChannelMux.Unlock()

	g.l.Info("got status not from map, interesting")
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
	g.l.Info("sending status as feedback to " + address.String())
	peerMessagesToSend <- addressedFeedbackStatus

	g.processRumorMessage(rmsg)
}

func (g *Gossiper) processRumorMessage(rmsg *RumorMessage) {
	isNewMessage := g.messageStorage.AddRumorMessage(rmsg)

	if isNewMessage {
		// rumormongering -- choose random peer to send rmsg to
		// can do optimization of not sending rumour to its sender, but it's not that necessary
		if g.arePeersEmpty() {
			g.l.Warn("no peers are known, cannot do rumor-mongering")
			return // msg came from client and no peers are known
		}

		g.spreadTheRumor(rmsg, nil)
	} else {
		// if we got an old rumor, then let's do nothing, we already have sent a feedback, everything is good
		g.l.Info("message is not new, skipping it")
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
		g.l.Warn("rumormongering with this peer is already in progress, this case is too hard for me")
		statusesChannelMux.Unlock()
		return
	}
	ch := make(chan *StatusPacket)
	statusesChannels[peer.String()] = ch
	statusesChannelMux.Unlock()

	g.l.Info("rumor sent further to " + peer.String())
	peerMessagesToSend <- &AddressedGossipPacket{Address: peer, Packet: &GossipPacket{Rumor: rmsg}}
	fmt.Println("MONGERING with " + peer.String())

	go g.startRumorMongeringThread(rmsg, ch, peer)
}

// --------------------------------------
// rumor-mongering thread section:
// --------------------------------------

func (g *Gossiper) startRumorMongeringThread(messageBeingRumored *RumorMessage, ch chan *StatusPacket, peer *UDPAddr) {
	ticker := time.NewTicker(RumorTimeout)
	select {
	case <-ticker.C:
		g.l.Info("peer " + peer.String() + " exceeded the timeout")

		// clean the map
		statusesChannelMux.Lock()
		delete(statusesChannels, peer.String())
		statusesChannelMux.Unlock()

		g.flipRumorMongeringCoin(messageBeingRumored)
	case statusPacket := <-ch:
		g.l.Info("processing status-response in rumor-mongering thread from " + peer.String())

		rmsg, otherHasSomethingNew := g.messageStorage.Diff(statusPacket)
		if rmsg != nil {
			g.l.Info("peer " + peer.String() + " doesn't know rmsg, sending it: " + rmsg.String())
			g.spreadTheRumor(rmsg, peer)
		} else if otherHasSomethingNew {
			g.l.Info("peer " + peer.String() + " knows more, than me, sending status to him")
			agp := &AddressedGossipPacket{Packet: &GossipPacket{Status: g.messageStorage.GetCurrentStatusPacket()}, Address: peer}
			peerMessagesToSend <- agp
		} else {
			fmt.Println("IN SYNC WITH " + peer.String())
			g.l.Info("peer " + peer.String() + " has same info as me, flipping the coin")
			g.flipRumorMongeringCoin(messageBeingRumored)
		}
	}
	// let the goroutine die!
}

func (g *Gossiper) flipRumorMongeringCoin(messageBeingRumored *RumorMessage) {
	continueMongering := (rand.Int() % 2) == 0
	if continueMongering {
		peer := g.getRandomPeer() // crutch because of fmt. requirements in HW1 :(
		g.l.Info("coin says to continue rumor-mongering")
		fmt.Println("FLIPPED COIN sending rumor to " + peer.String())
		g.spreadTheRumor(messageBeingRumored, peer)
	} else {
		g.l.Info("coin says to stop rumor-mongering of " + messageBeingRumored.String())
	}
}

// --------------------------------------
// end of rumor-mongering thread section
// --------------------------------------
