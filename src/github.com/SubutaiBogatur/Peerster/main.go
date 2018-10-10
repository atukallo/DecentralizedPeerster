package main

import (
	"flag"
	"fmt"
	. "github.com/SubutaiBogatur/Peerster/models"
	. "github.com/SubutaiBogatur/Peerster/utils"
	"github.com/dedis/protobuf"
	log "github.com/sirupsen/logrus"
	. "net"
	"strconv"
	"strings"
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

	g := Gossiper{}

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

	name string
}

func (g *Gossiper) startReadingConnection(conn *UDPConn, printCallback func(gp *GossipPacket)) {
	localAddress := conn.LocalAddr()
	logger.Info("starting reading bytes on " + localAddress.String())
	var buffer = make([]byte, MAX_PACKET_SIZE)

	for {
		n, _, _ := conn.ReadFrom(buffer)

		logger.Debug("read " + strconv.Itoa(n) + " bytes from " + localAddress.String() + ", decoding...")

		msg := &SimpleMessage{}
		err := protobuf.Decode(buffer, msg)
		if err != nil {
			logger.Warn("unable to decode message, error: " + err.Error())
		}

		logger.Info("read message from " + msg.RelayPeerAddr)
		logger.Debug("message is: " + msg.Contents)

		gp := &GossipPacket{Simple: msg}
		printCallback(gp)

		g.broadcastPacket(gp)
		logger.Info("Message processing is done")
	}
}

func (g *Gossiper) broadcastPacket(gp *GossipPacket) {
	osmsg := *gp.Simple // original
	nsmsg := osmsg      // new
	nsmsg.RelayPeerAddr = g.peersConnection.LocalAddr().String()

	relayPeer := osmsg.RelayPeerAddr
	relayPeerAddr, _ := ResolveUDPAddr("udp4", relayPeer)

	peerIsKnown := false
	for i := 0; i < len(g.peers); i++ {
		// not good cmp
		if g.peers[i].String() == relayPeerAddr.String() {
			peerIsKnown = true
			continue // not broadcasting him anything
		}

		g.sendSimpleMessage(&nsmsg, g.peers[i])
	}

	if !peerIsKnown {
		g.peers = append(g.peers, relayPeerAddr)
	}
}

func (g *Gossiper) sendSimpleMessage(smsg *SimpleMessage, addr *UDPAddr) {
	packetBytes, err := protobuf.Encode(smsg)
	if err != nil {
		logger.Error("client - unable to send msg: " + err.Error())
		return
	}

	logger.Info("sending message from " + g.peersConnection.LocalAddr().String() + " to " + addr.String())
	logger.Debug("msg is: " + string(smsg.Contents))

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
