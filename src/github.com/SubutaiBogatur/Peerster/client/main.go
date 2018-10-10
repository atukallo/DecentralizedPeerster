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
)

func initClient() (Client, error) {
	flag.Parse()

	c := Client{}

	{
		gossiperListenAddress := LocalIp + ":" + strconv.Itoa(*clientPort)
		gossiperListenUdpAddress, err := ResolveUDPAddr("udp4", gossiperListenAddress)
		if err != nil {
			log.Error("unable to parse clientListenAddress: " + string(gossiperListenAddress))
			return c, err
		}
		c.gossiperListenAddress = gossiperListenUdpAddress
	}

	// start listening (the client really does not need listening, but listening must be initiated before writing to socket)
	{
		gossiperUdpConn, err := ListenUDP("udp4", c.gossiperListenAddress)
		if err != nil {
			return c, err
		}
		c.gossiperListenConnection = gossiperUdpConn
	}

	return c, nil
}

type Client struct {
	gossiperListenAddress    *UDPAddr
	gossiperListenConnection *UDPConn
}

func (c *Client) sendMessageToGossiper(message string, gossiperPort *int) {
	msg := &Message{Text: message}
	packetBytes, err := protobuf.Encode(msg)
	if err != nil {
		log.Error("unable to send msg: " + err.Error())
		return
	}

	gossiperAddr, err := ResolveUDPAddr("udp4", LocalIp+":"+strconv.Itoa(*gossiperPort))
	if err != nil {
		log.Error("unable to send msg: " + err.Error())
		return
	}

	log.Info("sending message to " + gossiperAddr.String())
	log.Debug("msg is: " + string(message))

	n, err := c.gossiperListenConnection.WriteToUDP(packetBytes, gossiperAddr)
	if err != nil {
		log.Error("error when writing to connection: " + err.Error() + " n is " + strconv.Itoa(n))
	}

}


func main() {
	log.SetLevel(log.DebugLevel)

	c, err := initClient()
	if err != nil {
		log.Fatal("client failed to construct itself with error: " + err.Error())
		return
	}

	c.sendMessageToGossiper(*msg, UIPort)

	fmt.Println("client finished")
}
