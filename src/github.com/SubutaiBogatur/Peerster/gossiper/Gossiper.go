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
	Name atomic.Value // Name can be changed by webserver and is accessed from message-processor, thus using atomic string

	PeersAddress    *UDPAddr // PeersAddress for peers
	PeersConnection *UDPConn

	ClientAddress    *UDPAddr
	ClientConnection *UDPConn

	Peers         []*UDPAddr // accessed from message-processor and from rumor-mongering
	PeersSliceMux sync.Mutex

	MessageStorage *MessageStorage // accessed from message-processor and from rumor-mongering, is hard-synchronized

	IsSimpleMode bool       // in simple mode sending only simple messages
	L            *log.Entry // logger
}

// client-reader thread
func (g *Gossiper) StartClientReader() {
	g.L.Info("starting reading bytes on client: " + g.ClientAddress.String())
	var buffer = make([]byte, MaxPacketSize)

	for {
		g.ClientConnection.ReadFromUDP(buffer)

		cmsg := &ClientMessage{}
		if err := protobuf.Decode(buffer, cmsg); err != nil {
			// todo(atukallo): fix some protobuf warnings
			//g.L.Warn("unable to decode message, error: " + err.Error())
		}

		// ~~~ put into channel ~~~
		g.L.Debug("put client message into channel")
		clientMessagesToProcess <- cmsg
		// ~~~~~~~~~~~~~~~~~~~~~~~~
	}
}

// peer-reader thread
func (g *Gossiper) StartPeerReader() {
	g.L.Info("gossiper " + g.Name.Load().(string) +  ": starting reading bytes on peer: " + g.PeersAddress.String())
	var buffer = make([]byte, MaxPacketSize)

	for {
		_, addr, _ := g.PeersConnection.ReadFromUDP(buffer)

		gp := &GossipPacket{}
		if err := protobuf.Decode(buffer, gp); err != nil {
			// todo(atukallo): fix some protobuf warnings
			//g.L.Warn("unable to decode message, error: " + err.Error())
		}

		apg := &AddressedGossipPacket{Address: addr, Packet: gp}

		// ~~~ put into channel ~~~
		g.L.Debug("put peer message into channel")
		peerMessagesToProcess <- apg
		// ~~~~~~~~~~~~~~~~~~~~~~~~
	}
}

// peer-writer thread
func (g *Gossiper) StartPeerWriter() {
	g.L.Info("starting peer writer thread")

	for {
		// ~~~ read from channel ~~~
		agp := <-peerMessagesToSend
		// ~~~~~~~~~~~~~~~~~~~~~~~~~

		address := agp.Address
		gp := agp.Packet

		packetBytes, err := protobuf.Encode(gp)
		if err != nil {
			g.L.Error("unable to encode msg: " + err.Error())
			continue
		}

		g.L.Debug("sending message from " + g.PeersConnection.LocalAddr().String() + " to " + address.String())

		n, err := g.PeersConnection.WriteToUDP(packetBytes, address)
		if err != nil {
			g.L.Error("error when writing to connection: " + err.Error() + " n is " + strconv.Itoa(n))
			continue
		}
	}
}

func (g *Gossiper) StartAntiEntropyTimer() {
	g.L.Info("starting anti-entropy timer thread")

	for {
		ticker := time.NewTicker(AntiEntropyTimeout)
		<-ticker.C // wait for the timer to shoot

		if g.emptyPeers() {
			<-time.NewTicker(AntiEntropyTimeout * 5).C // wait additional time
			continue
		}

		// send status to a random peer
		peer := g.getRandomPeer()
		peerMessagesToSend <- &AddressedGossipPacket{Address: peer, Packet: &GossipPacket{Status: g.MessageStorage.GetCurrentStatusPacket()}}
	}
}

func (g *Gossiper) getRandomPeer() *UDPAddr {
	g.PeersSliceMux.Lock()
	defer g.PeersSliceMux.Unlock()

	randomInt := rand.Int31n(int32(len(g.Peers))) // end not inclusive
	randomPeer := g.Peers[randomInt]

	return randomPeer
}

func (g *Gossiper) emptyPeers() bool {
	g.PeersSliceMux.Lock()
	defer g.PeersSliceMux.Unlock()

	return len(g.Peers) == 0
}

func (g *Gossiper) GetPeersCopy() []*UDPAddr {
	g.PeersSliceMux.Lock()
	defer g.PeersSliceMux.Unlock()

	copySlice := make([]*UDPAddr, len(g.Peers))
	copy(copySlice, g.Peers)

	return copySlice
}

func (g *Gossiper) UpdatePeersIfNeeded(peer *UDPAddr) {
	g.PeersSliceMux.Lock()
	defer g.PeersSliceMux.Unlock()

	for i := 0; i < len(g.Peers); i++ {
		if g.Peers[i].String() == peer.String() {
			return // peer is a known one
		}
	}
	g.Peers = append(g.Peers, peer)
}

func (g *Gossiper) printPeers() {
	g.PeersSliceMux.Lock()
	defer g.PeersSliceMux.Unlock()

	fmt.Print("PEERS ")
	for i := 0; i < len(g.Peers); i++ {
		if i == len(g.Peers)-1 {
			fmt.Print(g.Peers[i].String())
		} else {
			fmt.Print(g.Peers[i].String() + ",")
		}
	}
	fmt.Println("")
}

// --------------------------------------
// message-processor thread section:
// --------------------------------------

func (g *Gossiper) StartMessageProcessor() {
	g.L.Info("starting message processor")

	for {
		select {
		case cmsg := <-clientMessagesToProcess:
			g.L.Debug("got client message from channel: " + cmsg.Text)
			cmsg.Print()
			g.printPeers()
			g.processClientMessage(cmsg)
		case agp := <-peerMessagesToProcess:
			g.L.Debug("got peer message from channel")
			agp.Print()
			g.printPeers()
			g.processAddressedGossipPacket(agp)
		}
	}
}

func (g *Gossiper) processClientMessage(cmsg *ClientMessage) {
	g.L.Info("got client message: " + cmsg.Text)
	gossiperName := g.Name.Load().(string)
	if !g.IsSimpleMode {
		messageId := g.MessageStorage.GetNextMessageId(gossiperName)
		rmsg := &RumorMessage{OriginalName: gossiperName, ID: messageId, Text: cmsg.Text}
		g.processRumorMessage(rmsg)
	} else {
		g.L.Info("mode is simple, so client message is distributed as simple one")
		smsg := &SimpleMessage{Text: cmsg.Text, OriginalName: gossiperName}
		g.processAddressedSimpleMessage(smsg, nil)
	}
}

func (g *Gossiper) processAddressedGossipPacket(agp *AddressedGossipPacket) {
	gp := agp.Packet
	address := agp.Address

	g.UpdatePeersIfNeeded(address)

	if gp.Rumor != nil {
		g.L.Info("got rumor-msg " + gp.Rumor.String() + " from " + address.String())
		g.processAddressedRumorMessage(gp.Rumor, address)
	} else if gp.Status != nil {
		g.L.Info("got status from " + address.String())
		g.processAddressedStatusPacket(gp.Status, address)
	} else if gp.Simple != nil {
		g.L.Info("got simple from " + address.String())
		g.processAddressedSimpleMessage(gp.Simple, address)
	}
}

func (g *Gossiper) processAddressedSimpleMessage(smsg *SimpleMessage, address *UDPAddr) {
	g.PeersSliceMux.Lock()
	defer g.PeersSliceMux.Unlock()

	smsg.RelayPeerAddr = g.PeersConnection.LocalAddr().String()

	for _, peer := range g.Peers {
		if address != nil && peer.String() == address.String() {
			continue
		}
		g.L.Info("sending simple to " + peer.String())
		peerMessagesToSend <- &AddressedGossipPacket{Address: peer, Packet: &GossipPacket{Simple: smsg}}
	}
}

func (g *Gossiper) processAddressedStatusPacket(sp *StatusPacket, address *UDPAddr) {
	statusesChannelMux.Lock()
	if val, isPresent := statusesChannels[address.String()]; isPresent {
		g.L.Info("status from " + address.String() + " found in map, forwarding it to corresponding goroutine...")
		val <- sp
		delete(statusesChannels, address.String()) // goroutine cannot eat more, than one status packet, so delete it from map
		statusesChannelMux.Unlock()
		return
	}
	statusesChannelMux.Unlock()

	g.L.Info("got status not from map, interesting")
	rmsg, otherHasSomethingNew := g.MessageStorage.Diff(sp)
	if rmsg != nil {
		g.spreadTheRumor(rmsg, address)
	} else if otherHasSomethingNew {
		peerMessagesToSend <- &AddressedGossipPacket{Packet: &GossipPacket{Status: g.MessageStorage.GetCurrentStatusPacket()}, Address: address}
	} else {
		log.Info("nothing interesting in the status")
		fmt.Println("IN SYNC WITH " + address.String())
	}
	// else do nothing at all
}

func (g *Gossiper) processAddressedRumorMessage(rmsg *RumorMessage, address *UDPAddr) {

	// send status back to rumorer:
	feedbackStatus := &GossipPacket{Status: g.MessageStorage.GetCurrentStatusPacket()}
	addressedFeedbackStatus := &AddressedGossipPacket{Address: address, Packet: feedbackStatus}
	g.L.Info("sending status as feedback to " + address.String())
	peerMessagesToSend <- addressedFeedbackStatus

	g.processRumorMessage(rmsg)
}

func (g *Gossiper) processRumorMessage(rmsg *RumorMessage) {
	isNewMessage := g.MessageStorage.AddRumorMessage(rmsg)

	if isNewMessage {
		// rumormongering -- choose random peer to send rmsg to
		if g.emptyPeers() {
			g.L.Warn("no Peers are known, cannot do rumor-mongering")
			return // msg came from client and no Peers are known
		}

		g.spreadTheRumor(rmsg, nil)
	} else {
		// if we got an old rumor, then let's do nothing, we already have sent a feedback, everything is good
		g.L.Info("message is not new, skipping it")
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
		g.L.Warn("rumormongering with this peer is already in progress, this case is too hard for me")
		statusesChannelMux.Unlock()
		return
	}
	ch := make(chan *StatusPacket)
	statusesChannels[peer.String()] = ch
	statusesChannelMux.Unlock()

	g.L.Info("rumor sent further to " + peer.String())
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
		g.L.Info("peer " + peer.String() + " exceeded the timeout")

		// clean the map
		statusesChannelMux.Lock()
		delete(statusesChannels, peer.String())
		statusesChannelMux.Unlock()

		g.flipRumorMongeringCoin(messageBeingRumored)
	case statusPacket := <-ch:
		g.L.Info("processing status-response in rumor-mongering thread from " + peer.String())

		rmsg, otherHasSomethingNew := g.MessageStorage.Diff(statusPacket)
		if rmsg != nil {
			g.L.Info("peer " + peer.String() + " doesn't know rmsg, sending it: " + rmsg.String())
			g.spreadTheRumor(rmsg, peer)
		} else if otherHasSomethingNew {
			g.L.Info("peer " + peer.String() + " knows more, than me, sending status to him")
			agp := &AddressedGossipPacket{Packet: &GossipPacket{Status: g.MessageStorage.GetCurrentStatusPacket()}, Address: peer}
			peerMessagesToSend <- agp
		} else {
			fmt.Println("IN SYNC WITH " + peer.String())
			g.L.Info("peer " + peer.String() + " has same info as me, flipping the coin")
			g.flipRumorMongeringCoin(messageBeingRumored)
		}
	}
	// let the goroutine die!
}

func (g *Gossiper) flipRumorMongeringCoin(messageBeingRumored *RumorMessage) {
	continueMongering := (rand.Int() % 2) == 0
	if continueMongering {
		peer := g.getRandomPeer() // crutch because of fmt. requirements in HW1 :(
		g.L.Info("coin says to continue rumor-mongering")
		fmt.Println("FLIPPED COIN sending rumor to " + peer.String())
		g.spreadTheRumor(messageBeingRumored, peer)
	} else {
		g.L.Info("coin says to stop rumor-mongering of " + messageBeingRumored.String())
	}
}

// --------------------------------------
// end of rumor-mongering thread section
// --------------------------------------
