package main

import (
	"flag"
	"fmt"
	. "github.com/SubutaiBogatur/Peerster/gossiper"
	. "github.com/SubutaiBogatur/Peerster/models"
	. "github.com/SubutaiBogatur/Peerster/utils"
	. "github.com/SubutaiBogatur/Peerster/webserver"
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
	peers      = flag.String("peers", "", "Other gossipers' addresses separated with \",\" in the form ip:port")
	simpleMode = flag.Bool("simple", false, "True, if mode is simple")
)

func initGossiper() (Gossiper, error) {
	flag.Parse()

	logger := log.WithField("bin", "gos")

	g := Gossiper{MessageStorage: &MessageStorage{VectorClock: make(map[string]uint32), Messages: make(map[string][]*RumorMessage)}}
	g.L = logger
	g.IsSimpleMode = *simpleMode

	clientListenAddress := LocalIp + ":" + strconv.Itoa(*UIPort)
	clientListenUdpAddress, err := ResolveUDPAddr("udp4", clientListenAddress)
	if err != nil {
		g.L.Error("Unable to parse clientListenAddress: " + string(clientListenAddress))
		return g, err
	}
	g.ClientAddress = clientListenUdpAddress

	peersListenAddress := *gossipAddr
	peersListenUdpAddress, err := ResolveUDPAddr("udp4", peersListenAddress)
	if err != nil {
		g.L.Error("Unable to parse peersListenAddress: " + string(peersListenAddress))
		return g, err
	}
	g.PeersAddress = peersListenUdpAddress
	g.L = g.L.WithField("a", peersListenUdpAddress.String())

	gossiperName := *name
	if gossiperName == "" {
		return g, PeersterError{ErrorMsg: "Empty name provided"}
	}
	g.Name = gossiperName

	peersArr := strings.Split(*peers, ",")
	for i := 0; i < len(peersArr); i++ {
		if peersArr[i] == "" {
			continue
		}

		peerAddr, err := ResolveUDPAddr("udp4", peersArr[i])
		if err != nil {
			g.L.Error("Error when parsing peers")
			return g, err
		}
		g.Peers = append(g.Peers, peerAddr)
	}

	// command line arguments parsed, start listening:
	peersUdpConn, err := ListenUDP("udp4", g.PeersAddress)
	if err != nil {
		return g, err
	}
	g.PeersConnection = peersUdpConn

	clientUdpConn, err := ListenUDP("udp4", g.ClientAddress)
	if err != nil {
		return g, err
	}
	g.ClientConnection = clientUdpConn

	return g, nil
}

func main() {
	log.SetLevel(log.DebugLevel)

	customFormatter := new(log.TextFormatter)
	//customFormatter.TimestampFormat = "15:04:05:05"
	//customFormatter.FullTimestamp = true
	log.SetFormatter(customFormatter)

	g, err := initGossiper()
	if err != nil {
		g.L.Fatal("gossiper failed to construct itself with error: " + err.Error())
		return
	}

	// set random seed
	rand.Seed(time.Now().Unix())

	go g.StartClientReader()
	go g.StartPeerReader()
	go g.StartPeerWriter()
	go g.StartAntiEntropyTimer()
	go StartWebserver(&g)

	g.StartMessageProcessor() // goroutine dies, when app dies, so blocking function is called in main thread

	fmt.Println("gossiper finished")
}
