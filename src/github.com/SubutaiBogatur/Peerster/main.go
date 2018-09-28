package main

import (
	"flag"
	"fmt"
	"net"
)

const (
	localIp = "127.0.0.1" // ip addr of gossiper through loopback interface
)

// command line arguments
var (
	UIPort       = flag.Int("UIPort", 4848, "Port, where gossiper listens for client. Client is situated on the same machine, so gossiper listens to " + localIp + ":port for client")
	gossipAddr   = flag.String("gossipAddr", "127.0.0.1:1212", "Address, where gossiper is launched: ip:port. Other peers will contact gossiper through this address")
	name         = flag.String("name", "go_rbachev", "Gossiper name")
	initialPeers = flag.String("peers", "", "Other gossipers' addresses separated with \",\" in the form ip:port")
	simple       = flag.Bool("simple", true, "True, if mode is simple")

	//allPeers = []string
)

type SimpleMessage struct {
	OriginalName  string // name of original sender (why not address?)
	RelayPeerAddr string // address of last retranslator in the form ip:port
	Contents      string
}

type GossipPacket struct {
	Simple *SimpleMessage
}

func printPackage(pckg *GossipPacket, isFromClient bool) {
	var smsg *SimpleMessage = pckg.Simple

	if isFromClient {
		fmt.Println("CLIENT MESSAGE " + smsg.Contents)
	} else {
		fmt.Println("SIMPLE MESSAGE origin " + smsg.OriginalName + " from " + smsg.RelayPeerAddr + " contents " + smsg.Contents)
	}

	fmt.Println("PEERS todo:")
	//fmt.Println(allPeers)
}

func validateCommandLineArguments() {
	clientListenAddress := localIp + ":" + string(*UIPort)
	clientListenUdpAddress, err := net.ResolveUDPAddr("udp4", clientListenAddress)
	if err != nil {
		fmt.Println("Unable to parse clientListenAddress: " + clientListenAddress + ", error is " + err)
	}

	peersListenAddress := gossipAddr
	peersListenUdpAddress, err := net.ResolveUDPAddr("udp4", peersListenAddress)
	if err != nil {
		fmt.Println("Unable to parse peersListenAddress: " + peersListenAddress + ", error is " + err)
	}


	// todo: validate peers
}

type Gossiper struct {
	address *net.UDPAddr
	name string
}

//func NewGossiper()

func (g Gossiper) startListening() {

}


func main() {
	flag.Parse()
	//fmt.Println(*simple)
	//smsg := &SimpleMessage{"aha", "haha", "contents"}
	//gp := GossipPacket{smsg}

	//"127.0.0.1:" + UIPort
	//gossipAddr

	address := "127.0.0.1:5151"


	udpAddr, err := net.ResolveUDPAddr("udp4", address)
	udpConn, err := net.ListenUDP("udp4", udpAddr)

	for {
		udpConn.Read()
	}


	//udpConn.
	//fmt.Println(*udpConn)


	fmt.Println(err)
}
