package main

import (
	"flag"
	"fmt"
	log "github.com/sirupsen/logrus"
	. "net"
	"strconv"
)

const (
	localIp = "127.0.0.1" // ip addr of gossiper through loopback interface

	MAX_PACKET_SIZE = 2048 // in bytes
)

// command line arguments
var (
	UIPort = flag.Int("UIPort", 4848, "Port, where client is launched. Client is launched on 127.0.0.1:{port}")
	msg    = flag.String("msg", "", "Message to send to gossiper")
)

func initClient() (Client, error) {
	flag.Parse()

	c := Client{}

	{
		gossiperListenAddress := localIp + ":" + strconv.Itoa(*UIPort) // the client really does not listening, but listening must be enabled before writing to socket
		gossiperListenUdpAddress, err := ResolveUDPAddr("udp4", gossiperListenAddress)
		if err != nil {
			log.Error("Unable to parse clientListenAddress: " + string(gossiperListenAddress))
			return c, err
		}
		c.gossiperAddress = gossiperListenUdpAddress
	}

	// command line arguments parsed, start listening:
	{
		gossiperUdpConn, err := ListenUDP("udp4", c.gossiperAddress)
		if err != nil {
			return c, err
		}
		c.gossiperConnection = gossiperUdpConn
	}

	return c, nil
}

type Client struct {
	gossiperAddress    *UDPAddr
	gossiperConnection *UDPConn
}

//func (*Client) sendMessage()

func main() {
	c, err := initClient()
	if err != nil {
		log.Fatal("client failed to construct itself with error: " + err.Error())
		return
	}



	fmt.Println("client ran")
}
