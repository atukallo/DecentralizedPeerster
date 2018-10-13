package main

import (
	"flag"
	"fmt"
	. "github.com/SubutaiBogatur/Peerster/gossiper"
	. "github.com/SubutaiBogatur/Peerster/utils"
	. "github.com/SubutaiBogatur/Peerster/webserver"
	log "github.com/sirupsen/logrus"
	"math/rand"
	"time"
)

// command line arguments
var (
	uiport     = flag.Int("UIPort", 4848, "Port, where gossiper listens for client. Client is situated on the same machine, so gossiper listens to "+LocalIp+":port for client")
	gossipAddr = flag.String("gossipAddr", "127.0.0.1:1212", "Address, where gossiper is launched: ip:port. Other peers will contact gossiper through this peersAddress")
	name       = flag.String("name", "go_rbachev", "Gossiper name")
	peers      = flag.String("peers", "127.0.0.1:1212", "Other gossipers' addresses separated with \",\" in the form ip:port")
	simpleMode = flag.Bool("simple", false, "True, if mode is simple")
	webserver  = flag.Bool("webserver", true, "False, if webserver is not needed")
)

func main() {
	log.SetLevel(log.FatalLevel)

	customFormatter := new(log.TextFormatter)
	//customFormatter.TimestampFormat = "15:04:05:05"
	//customFormatter.FullTimestamp = true
	log.SetFormatter(customFormatter)

	flag.Parse()

	g, err := NewGossiper(*name, *uiport, *gossipAddr, *peers, *simpleMode)
	if err != nil {
		log.Fatal("gossiper failed to construct itself with error: " + err.Error())
		return
	}

	// set random seed
	rand.Seed(time.Now().Unix())

	go g.StartClientReader()
	go g.StartPeerReader()
	go g.StartPeerWriter()
	go g.StartAntiEntropyTimer()

	if *webserver {
		go StartWebserver(g)
	}

	g.StartMessageProcessor() // goroutine dies, when app dies, so blocking function is called in main thread

	fmt.Println("gossiper finished")
}
