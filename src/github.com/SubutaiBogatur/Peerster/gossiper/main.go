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
	peers      = flag.String("peers", "", "Other gossipers' addresses separated with \",\" in the form ip:port")
	simple     = flag.Bool("simple", true, "True, if mode is simple")

	additional = flag.String("additional", "", "additional info");
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
			log.Error("Unable to parse peersListenAddress: " + string(peersListenAddress))
			return g, err
		}
		g.peersAddress = peersListenUdpAddress
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
			peerAddr, err := ResolveUDPAddr("udp4", peersArr[i])
			if err != nil {
				log.Error("Error when parsing peers")
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

func (g *Gossiper) startReadingConnection(conn *UDPConn) {
	localAddress := conn.LocalAddr()
	log.Info("starting reading bytes on " + localAddress.String())
	var buffer = make([]byte, MAX_PACKET_SIZE)

	for {
		n, _, _ := conn.ReadFrom(buffer)

		log.Info("Read " + strconv.Itoa(n) + " bytes from " + localAddress.String() + ", decoding...")

		msg := &Message{}
		err := protobuf.Decode(buffer, msg)
		if err != nil {
			log.Warn("Unable to decode message, error: " + err.Error())
		}

		log.Info("Read message: " + msg.Text)
	}
}

func (g *Gossiper) startReadingPeers() {
	log.Info("starting reading bytes from peers on " + g.peersAddress.String())
	g.startReadingConnection(g.peersConnection)
}

func (g *Gossiper) startReadingClient() {
	log.Info("starting reading bytes from client on " + g.clientAddress.String())
	g.startReadingConnection(g.clientConnection)
}

func main() {
	g, err := initGossiper()
	if err != nil {
		log.Fatal("gossiper failed to construct itself with error: " + err.Error())
		return
	}

	go g.startReadingPeers()
	g.startReadingClient() // goroutine dies, when app dies, so blocking function is called in main thread

	fmt.Println("gossiper finished")
}
