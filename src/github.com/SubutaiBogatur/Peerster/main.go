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
// * client-reader     thread : the only port reading from client socket. Sends client message to message-processor
// * peer-reader       thread : the only port reading from peer socket. Sends GossipPackets to message-processor
// * peer-writer       thread : the only port writing to peer socket. Listens to channel for GossipPackets and writes them
// * peer-pinger       thread : tbi in antientropy
// * message-processor thread : is an abstraction on client-listener, peer-listener threads. Receives all the messages and for every message:
//     + rumor-msg  : upd status, send status back, send rumor further, start rumor-mongering thread waiting for status (what if already exists for this peer??)
//     + status-msg : push it to one of the rumor-mongering threads (what if not present??)
// * rumor-mongering   thread : thread waits either for status-msg to arrive or for timeout and stores rumor-msg, it was initiated for
//     + status-msg : cmp (store sync-safe Map for VectorClock, which are edited from message-processor) and send new msg via peer-communicator
//     + timeout    : 1/2 & send new rumor-msg via peer-communicator

// command line arguments
var (
	UIPort     = flag.Int("UIPort", 4848, "Port, where gossiper listens for client. Client is situated on the same machine, so gossiper listens to "+LocalIp+":port for client")
	gossipAddr = flag.String("gossipAddr", "127.0.0.1:1212", "Address, where gossiper is launched: ip:port. Other peers will contact gossiper through this peersAddress")
	name       = flag.String("name", "go_rbachev", "Gossiper name")
	peers      = flag.String("peers", "127.0.0.1:1212", "Other gossipers' addresses separated with \",\" in the form ip:port")
	simple     = flag.Bool("simple", true, "True, if mode is simple")

	additional = flag.String("additional", "", "additional info")

	logger = log.WithField("bin", "gos")

	clientMessagesToProcess = make(chan *ClientMessage)
	peerMessagesToProcess   = make(chan *AddressedGossipPacket)
	peerMessagesToSend      = make(chan *AddressedGossipPacket)

	// map is accessed from message-processor and from rumor-mongering threads, should be converted to sync.Map{}
	statusesChannels   = make(map[string]chan *StatusPacket) // peerIp -> channel, where rumor-mongering goroutine is waiting for status feedback
	statusesChannelMux sync.Mutex
)

const (
	RumorTimeout = time.Second
)

func initGossiper() (Gossiper, error) {
	flag.Parse()

	g := Gossiper{messageStorage: &MessageStorage{VectorClock: make(map[string]uint32), Messages: make(map[string][]*RumorMessage)}}

	{
		clientListenAddress := LocalIp + ":" + strconv.Itoa(*UIPort)
		clientListenUdpAddress, err := ResolveUDPAddr("udp4", clientListenAddress)
		if err != nil {
			logger.Error("Unable to parse clientListenAddress: " + string(clientListenAddress))
			return g, err
		}
		g.clientAddress = clientListenUdpAddress
	}

	{
		peersListenAddress := *gossipAddr
		peersListenUdpAddress, err := ResolveUDPAddr("udp4", peersListenAddress)
		if err != nil {
			logger.Error("Unable to parse peersListenAddress: " + string(peersListenAddress))
			return g, err
		}
		g.peersAddress = peersListenUdpAddress
		logger = logger.WithField("a", peersListenUdpAddress.String())
	}

	{
		gossiperName := *name
		if gossiperName == "" {
			return g, PeersterError{ErrorMsg: "Empty name provided"}
		}
		g.name = gossiperName
	}

	{
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
	}

	// command line arguments parsed, start listening:
	{
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
	}

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

		logger.Info("sending message from " + g.peersConnection.LocalAddr().String() + " to " + address.String())

		n, err := g.peersConnection.WriteToUDP(packetBytes, address)
		if err != nil {
			logger.Error("error when writing to connection: " + err.Error() + " n is " + strconv.Itoa(n))
			continue
		}
	}
}

// --------------------------------------
// message-processor thread section:
// --------------------------------------

func (g *Gossiper) startMessageProcessor() {
	logger.Info("starting message processor")

	for {
		select {
		case cmsg := <-clientMessagesToProcess:
			logger.Info("got client message from channel: " + cmsg.Text)
			g.processClientMessage(cmsg)
		case agp := <-peerMessagesToProcess:
			logger.Info("got peer message from channel")
			g.processAddressedGossipPacket(agp)
		}
	}
}

func (g *Gossiper) processClientMessage(cmsg *ClientMessage) {
	messageId := g.messageStorage.GetNextMessageId(g.name)
	rmsg := &RumorMessage{OriginalName: g.name, ID: messageId, Text: cmsg.Text}
	g.processRumorMessage(rmsg)
}

func (g *Gossiper) processAddressedGossipPacket(agp *AddressedGossipPacket) {
	gp := agp.Packet
	address := agp.Address

	g.updatePeersIfNeeded(address)

	if gp.Rumor != nil {
		g.processAddressedRumorMessage(gp.Rumor, address)
	} else if gp.Status != nil {
		g.processAddressedStatusPacket(gp.Status, address)
	} else if gp.Simple != nil {
		logger.Warn("someone is sending simple messages, ahahahah")
	}
}

func (g *Gossiper) processAddressedStatusPacket(sp *StatusPacket, address *UDPAddr) {
	statusesChannelMux.Lock()
	if val, isPresent := statusesChannels[address.String()]; isPresent {
		logger.Info("got status from map, okidoki")
		val <- sp // here is a small non-critical race condition (the sp we write may not be read at all, if the eating goroutine has not cleaned the map yet)
		statusesChannelMux.Unlock()
		return
	}
	statusesChannelMux.Unlock()

	logger.Info("got status not from map, interesting")
	rmsg, otherHasSomethingNew := g.messageStorage.Diff(sp)
	if rmsg != nil {
		g.spreadTheRumor(rmsg, address)
	} else if otherHasSomethingNew {
		agp := &AddressedGossipPacket{Packet: &GossipPacket{Status: g.messageStorage.GetCurrentStatusPacket()}, Address: address}
		peerMessagesToSend <- agp
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
		if len(g.peers) == 0 { // access without sync, but nothing can go wrong
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
		g.peersSliceMux.Lock()
		randomInt := rand.Int31n(int32(len(g.peers))) // end not inclusive
		peer = g.peers[randomInt]
		g.peersSliceMux.Unlock()
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
		g.flipRumorMongeringCoin(messageBeingRumored)
	case statusPacket := <-ch:
		logger.Info("peer " + peer.String() + " sent status as response")

		// clean the map
		statusesChannelMux.Lock()
		delete(statusesChannels, peer.String())
		statusesChannelMux.Unlock()

		rmsg, otherHasSomethingNew := g.messageStorage.Diff(statusPacket)
		if rmsg != nil {
			logger.Info("peer " + peer.String() + " doesn't know rmsg, sending it: " + rmsg.String())
			g.spreadTheRumor(rmsg, peer)
		} else if otherHasSomethingNew {
			logger.Info("peer " + peer.String() + " knows more, than me, sending status to him")
			agp := &AddressedGossipPacket{Packet: &GossipPacket{Status: g.messageStorage.GetCurrentStatusPacket()}, Address: peer}
			peerMessagesToSend <- agp
		} else {
			logger.Info("peer " + peer.String() + " has same info as me, flipping the coin")
			g.flipRumorMongeringCoin(messageBeingRumored)
		}
	}
	// let the goroutine die!
}

func (g *Gossiper) flipRumorMongeringCoin(messageBeingRumored *RumorMessage) {
	continueMongering := (rand.Int() % 2) == 0
	if continueMongering {
		g.spreadTheRumor(messageBeingRumored, nil)
	} else {
		logger.Info("finished mongering of " + messageBeingRumored.String())
	}
}

// --------------------------------------
// end of rumor-mongering thread section
// --------------------------------------

func main() {
	g, err := initGossiper()
	if err != nil {
		logger.Fatal("gossiper failed to construct itself with error: " + err.Error())
		return
	}

	go g.startClientReader()
	go g.startPeerReader()
	go g.startPeerWriter()

	g.startMessageProcessor() // goroutine dies, when app dies, so blocking function is called in main thread

	fmt.Println("gossiper finished")
}
