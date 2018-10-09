package main

import (
	"flag"
	"fmt"
	"github.com/dedis/protobuf"
	. "net"
	"strconv"
)

const (
	localIp = "127.0.0.1" // ip addr of gossiper through loopback interface

	MAX_PACKET_SIZE = 2048 // in bytes
)

// command line arguments
var (
	UIPort     = flag.Int("UIPort", 4848, "Port, where gossiper listens for client. Client is situated on the same machine, so gossiper listens to "+localIp+":port for client")
	gossipAddr = flag.String("gossipAddr", "127.0.0.1:1212", "Address, where gossiper is launched: ip:port. Other peers will contact gossiper through this peersAddress")
	name       = flag.String("name", "go_rbachev", "Gossiper name")
	peers      = flag.String("peers", "", "Other gossipers' addresses separated with \",\" in the form ip:port")
	simple     = flag.Bool("simple", true, "True, if mode is simple")

	additional = flag.String("additional", "", "additional info");
)

type SimpleMessage struct {
	OriginalName  string // name of original sender (why not peersAddress?)
	RelayPeerAddr string // peersAddress of last retranslator in the form ip:port
	Contents      string
}

type Message struct {
	Text string
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

func initGossiper() (Gossiper, error) {
	flag.Parse()

	g := Gossiper{}

	clientListenAddress := localIp + ":" + strconv.Itoa(*UIPort)

	clientListenUdpAddress, err := ResolveUDPAddr("udp4", clientListenAddress)
	if err != nil {
		fmt.Println("Unable to parse clientListenAddress: " + string(clientListenAddress) + ", error is " + err.Error())
		return g, err

		fmt.Println(clientListenUdpAddress) // todo: tmp
	}

	peersListenAddress := *gossipAddr
	peersListenUdpAddress, err := ResolveUDPAddr("udp4", peersListenAddress)
	if err != nil {
		fmt.Println("Unable to parse peersListenAddress: " + string(peersListenAddress) + ", error is " + err.Error())
		return g, err
	}
	g.peersAddress = peersListenUdpAddress

	gossiperName := *name
	if gossiperName == "" {
		return g, PeersterError{errorMsg: "Empty name provided"}
	}
	g.name = gossiperName

	// todo: validate peers

	// command line arguments parsed, start listening:
	udpConn, err := ListenUDP("udp4", g.peersAddress)
	if err != nil {
		return g, err
	}
	g.peerConnection = udpConn

	return g, nil
}

type Gossiper struct {
	peersAddress   *UDPAddr // peersAddress for peers
	peerConnection *UDPConn
	name           string
}

func (g *Gossiper) startReadingBytes() {
	fmt.Println("start reading bytes")
	for i := 0; i < 10; i++ {
		var buffer = make([]byte, MAX_PACKET_SIZE)

		g.peerConnection.ReadFrom(buffer)

		fmt.Println("Read buffer:")
		//fmt.Println(buffer[0:n])

		msg := &Message{}
		err := protobuf.Decode(buffer, msg)
		if err != nil {
			fmt.Println("Error is " + err.Error())
		}

		fmt.Println(msg)
	}
}

func (g *Gossiper) sendMessageTo(addr string) {
	msg := &Message{Text: "hau hi"}
	packetBytes, err := protobuf.Encode(msg)
	if err != nil {
		fmt.Println(packetBytes)
		return
	}

	udpAddr, err := ResolveUDPAddr("udp4", addr)

	fmt.Println("sending message to: " + addr)

	if err != nil {
		fmt.Println(packetBytes)
		return
	}

	fmt.Println(packetBytes)

	g.peerConnection.WriteToUDP(packetBytes, udpAddr)
}

func main() {
	g, err := initGossiper()
	if err != nil {
		fmt.Println("gossiper failed to construct itself with error: " + err.Error())
		return
	}

	if *additional != "" {
		// than additional is ip address of where to send
		g.sendMessageTo(*additional)
	} else {
		// passive mode
		g.startReadingBytes() // locking operation
	}

	fmt.Println("gossiper ran")
}

type PeersterError struct {
	errorMsg string
}

func (e PeersterError) Error() string {
	return e.errorMsg
}
