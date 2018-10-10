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
	clientPort = flag.Int("clientPort", 4747, "Port, where client is launched")
	msg        = flag.String("msg", "tmptmp", "Message to send to gossiper")

	logger = log.WithField("bin", "clt")
)

func initClient() (Client, error) {
	flag.Parse()

	c := Client{}

	{
		gossiperListenAddress := LocalIp + ":" + strconv.Itoa(*clientPort)
		gossiperListenUdpAddress, err := ResolveUDPAddr("udp4", gossiperListenAddress)
		if err != nil {
			logger.Error("unable to parse clientListenAddress: " + string(gossiperListenAddress))
			return c, err
		}
		c.gossiperListenAddress = gossiperListenUdpAddress
		logger = logger.WithField("a", gossiperListenUdpAddress.String())
	}

	// start listening (the client really does not need listening, but listening must be initiated before writing to socket)
	{
		gossiperUdpConn, err := ListenUDP("udp4", c.gossiperListenAddress)
		if err != nil {
			return c, err
		}
		c.gossiperListenConnection = gossiperUdpConn
		logger.Info("listening on " + c.gossiperListenAddress.String())
	}

	return c, nil
}

type Client struct {
	gossiperListenAddress    *UDPAddr
	gossiperListenConnection *UDPConn
}

func (c *Client) sendMessageToGossiper(message string, gossiperPort *int) {
	msg := &SimpleMessage{OriginalName: "atukallo_client", RelayPeerAddr:c.gossiperListenConnection.LocalAddr().String(), Contents:message}
	packetBytes, err := protobuf.Encode(msg)
	if err != nil {
		logger.Error("unable to send msg: " + err.Error())
		return
	}

	fmt.Println(msg.OriginalName)

	gossiperAddr, err := ResolveUDPAddr("udp4", LocalIp+":"+strconv.Itoa(*gossiperPort))
	if err != nil {
		logger.Error("unable to send msg: " + err.Error())
		return
	}

	logger.Info("sending message to " + gossiperAddr.String())
	logger.Debug("msg is: " + string(message))

	n, err := c.gossiperListenConnection.WriteToUDP(packetBytes, gossiperAddr)
	if err != nil {
		logger.Error("error when writing to connection: " + err.Error() + " n is " + strconv.Itoa(n))
	}
}


func main() {
	log.SetLevel(log.DebugLevel)

	c, err := initClient()
	if err != nil {
		logger.Fatal("client failed to construct itself with error: " + err.Error())
		return
	}

	c.sendMessageToGossiper(*msg, UIPort)

	logger.Info("work done, shutting down")
	fmt.Println("client finished")
}
