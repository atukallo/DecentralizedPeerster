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
)

// command line arguments
var (
	UIPort     = flag.Int("UIPort", 4848, "Port, where gossiper is listening for a client. Gossiper is listening on 127.0.0.1:{port}")
	msg        = flag.String("msg", "tmptmp", "Message to send to gossiper")

	logger = log.WithField("bin", "clt")
)

func sendMessageToGossiper(message string, gossiperPort *int) {
	msg := &ClientMessage{Text:message}
	packetBytes, err := protobuf.Encode(msg)
	if err != nil {
		logger.Error("unable to send msg: " + err.Error())
		return
	}

	gossiperAddr, err := ResolveUDPAddr("udp4", LocalIp+":"+strconv.Itoa(*gossiperPort))
	if err != nil {
		logger.Error("unable to send msg: " + err.Error())
		return
	}

	logger.Info("sending message to " + gossiperAddr.String())
	logger.Debug("msg is: " + string(message))

	connToGossiper, err := Dial("udp4", gossiperAddr.String())
	if err != nil {
		logger.Error("error dialing: " + err.Error())
	}

	n, err := connToGossiper.Write(packetBytes)
	if err != nil {
		logger.Error("error when writing to connection: " + err.Error() + " n is " + strconv.Itoa(n))
	}
}


func main() {
	log.SetLevel(log.DebugLevel)

	sendMessageToGossiper(*msg, UIPort)

	logger.Info("work done, shutting down")
	fmt.Println("client finished")
}
