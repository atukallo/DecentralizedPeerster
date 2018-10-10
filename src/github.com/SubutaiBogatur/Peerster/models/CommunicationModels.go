package models

import (
	"fmt"
	"net"
)

type SimpleMessage struct {
	OriginalName  string // name of original sender (why not peersAddress?)
	RelayPeerAddr string // peersAddress of last retranslator in the form ip:port
	Contents      string
}

//type Message struct {
//	Text string
//}

type GossipPacket struct {
	Simple *SimpleMessage
}

func (gp *GossipPacket) PrintClientPacket(knownPeers []*net.UDPAddr) {
	gp.printPackage(knownPeers, true)
}

func (gp *GossipPacket) PrintPeerPacket(knownPeers []*net.UDPAddr) {
	gp.printPackage(knownPeers, false)
}

func (gp *GossipPacket) printPackage(knownPeers []*net.UDPAddr, isFromClient bool) {
	var smsg *SimpleMessage = gp.Simple

	if isFromClient {
		fmt.Println("CLIENT MESSAGE " + smsg.Contents)
	} else {
		fmt.Println("SIMPLE MESSAGE origin " + smsg.OriginalName + " from " + smsg.RelayPeerAddr + " contents " + smsg.Contents)
	}

	fmt.Print("PEERS ")
	for i := 0; i < len(knownPeers); i++ {
		if i == len(knownPeers)-1 {
			fmt.Print(knownPeers[i].String())
		} else {
			fmt.Print(knownPeers[i].String() + ",")
		}
	}
	fmt.Println("")
}
