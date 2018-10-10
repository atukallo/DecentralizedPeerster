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
	"time"
)

// command line arguments
var (
	UIPort     = flag.Int("UIPort", 4848, "Port, where gossiper listens for client. Client is situated on the same machine, so gossiper listens to "+LocalIp+":port for client")
	gossipAddr = flag.String("gossipAddr", "127.0.0.1:1212", "Address, where gossiper is launched: ip:port. Other peers will contact gossiper through this peersAddress")
	name       = flag.String("name", "go_rbachev", "Gossiper name")
	peers      = flag.String("peers", "127.0.0.1:1212", "Other gossipers' addresses separated with \",\" in the form ip:port")
	simple     = flag.Bool("simple", true, "True, if mode is simple")

	additional = flag.String("additional", "", "additional info")

	logger = log.WithField("bin", "gos")
)

func initGossiper() (Gossiper, error) {
	flag.Parse()

	g := Gossiper{messageStorage: &MessageStorage{VectorClock: make(map[string]uint32), Messages: make(map[string][]*RumorMessage)}}

	{
		clientListenAddress := LocalIp + ":" + strconv.Itoa(*UIPort)
		clientListenUdpAddress, err := ResolveUDPAddr("udp4", clientListenAddress)
		if err != nil {
			log.Error("Unable to parse clientListenAddress: " + string(clientListenAddress))
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

	peers []*UDPAddr

	messageStorage *MessageStorage

	name string
}

// todo: add synchronization, because MessageStorage is definitely not-concurrent safe
func (g *Gossiper) startReadingConnection(conn *UDPConn, isClient bool) {
	localAddress := conn.LocalAddr()
	logger.Info("starting reading bytes on " + localAddress.String())
	var buffer = make([]byte, MAX_PACKET_SIZE)

	for {
		n, senderAddr, _ := conn.ReadFromUDP(buffer)
		g.updatePeers(senderAddr)

		logger.Debug("read " + strconv.Itoa(n) + " bytes from " + localAddress.String() + ", decoding...")

		if isClient {
			cmsg := &ClientMessage{}
			if err := protobuf.Decode(buffer, cmsg); err != nil {
				logger.Warn("unable to decode message, error: " + err.Error())
			}
			g.processClientMessage(cmsg)
		} else {
			gp := &GossipPacket{}
			if err := protobuf.Decode(buffer, gp); err != nil {
				logger.Warn("unable to decode message, error: " + err.Error())
			}

			if gp.Rumor != nil {
				g.processRumorMessage(gp.Rumor, senderAddr)
			} else if gp.Status != nil {
				g.processStatusPacket(gp.Status, senderAddr)
			} else if gp.Simple != nil {
				g.processStatusPacket(gp.Status, senderAddr)
			} else {
				log.Error("unable to parse gossip packet! All fields are empty")
			}
		}

		logger.Info("message processing is done")
	}
}

func (g *Gossiper) processGossipPacket(gp *GossipPacket, isFromClient bool) {
	osmsg := *gp.Simple // original
	nsmsg := osmsg      // new
	nsmsg.RelayPeerAddr = g.peersConnection.LocalAddr().String()

	if isFromClient {
		nsmsg.OriginalName = g.name
	}

	relayPeer := osmsg.RelayPeerAddr
	relayPeerAddr, _ := ResolveUDPAddr("udp4", relayPeer)

	peerIsKnown := false
	for i := 0; i < len(g.peers); i++ {
		// not good cmp
		if g.peers[i].String() == relayPeerAddr.String() && !isFromClient {
			peerIsKnown = true
			continue // not broadcasting him anything
		}

		g.sendSimpleMessage(&nsmsg, g.peers[i])
	}

	if !peerIsKnown && !isFromClient {
		g.peers = append(g.peers, relayPeerAddr)
	}
}

func (g *Gossiper) processClientMessage(cmsg *ClientMessage) {
	rmsg := &RumorMessage{OriginalName: g.name, ID: g.messageStorage.GetNextMessageId(g.name), Text: cmsg.Text}
	g.processRumorMessage(rmsg, nil)
}

func (g *Gossiper) processRumorMessage(rmsg *RumorMessage, senderAddr *UDPAddr) {
	isNewMessage := g.messageStorage.ProcessRumorMessage(rmsg)

	if isNewMessage {
		// rumormongering -- choose random peer to send rmsg to
		randomInt := rand.Int31n(int32(len(g.peers))) // end not inclusive
		randomPeer := g.peers[randomInt]
		g.sendRumourMessage(rmsg, randomPeer)
	} else {
		// todo
	}

}

func (g *Gossiper) processStatusPacket(sp *StatusPacket, senderAddr *UDPAddr) {
}

func (g *Gossiper) processSimpleMessage(smsg *SimpleMessage) {

}

func (g *Gossiper) updatePeers(peer *UDPAddr) {
	// todo: change peers to set

	if peer == nil {
		return
	}

	for i := 0; i < len(g.peers); i++ {
		// todo: bad cmp
		if g.peers[i].String() == peer.String() {
			return // peer is a known one
		}
	}

	g.peers = append(g.peers, peer)
}

func (g* Gossiper) sendRumourMessage(rmsg *RumorMessage, addr *UDPAddr) {
	log.Info("sending rumour message with text: " + rmsg.Text)
	g.sendData(rmsg, addr)
}

func (g *Gossiper) sendSimpleMessage(smsg *SimpleMessage, addr *UDPAddr) {
	log.Info("sending simple message with text: " + smsg.Text)
	g.sendData(smsg, addr)
}

func (g *Gossiper) sendData(data interface{}, addr *UDPAddr) {
	packetBytes, err := protobuf.Encode(data)
	if err != nil {
		logger.Error("unable to send msg: " + err.Error())
		return
	}

	logger.Info("sending message from " + g.peersConnection.LocalAddr().String() + " to " + addr.String())

	n, err := g.peersConnection.WriteToUDP(packetBytes, addr)
	if err != nil {
		logger.Error("error when writing to connection: " + err.Error() + " n is " + strconv.Itoa(n))
	}
}


func (g *Gossiper) startReadingPeers() {
	logger.Info("starting reading bytes from peers on " + g.peersAddress.String())
	g.startReadingConnection(g.peersConnection, func(gp *GossipPacket) {
		gp.PrintPeerPacket(g.peers) // todo: will it change at runtime? probably should
	})
}

func (g *Gossiper) startReadingClient() {
	logger.Info("starting reading bytes from client on " + g.clientAddress.String())
	g.startReadingConnection(g.clientConnection, func(gp *GossipPacket) {
		gp.PrintClientPacket(g.peers)
	})
}

func main() {
	g, err := initGossiper()
	if err != nil {
		logger.Fatal("gossiper failed to construct itself with error: " + err.Error())
		return
	}

	go g.startReadingPeers()
	g.startReadingClient() // goroutine dies, when app dies, so blocking function is called in main thread

	fmt.Println("gossiper finished")
}
