package models

import (
	"fmt"
	"net"
)

type AdressedGossipPacket struct {
	Packet  *GossipPacket
	Address *net.UDPAddr
}

// the invariant on the packet is that only one of the fields is not nil
type GossipPacket struct {
	Simple *SimpleMessage
	Rumor  *RumorMessage
	Status *StatusPacket
}

type SimpleMessage struct {
	OriginalName  string // name of original gossiper sender
	RelayPeerAddr string // address of latest peer retranslator in the form ip:port
	Text          string
}

type RumorMessage struct {
	OriginalName string // name of original gossiper sender
	ID           uint32 // id assigned by original sender ie counter per sender
	Text         string
}

type StatusPacket struct {
	Want []PeerStatus // vector clock
}

// depicts gossiper's information about another peer
type PeerStatus struct {
	Identifier string
	NextID     uint32
}

type ClientMessage struct {
	Text string
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
		fmt.Println("CLIENT MESSAGE " + smsg.Text)
	} else {
		fmt.Println("SIMPLE MESSAGE origin " + smsg.OriginalName + " from " + smsg.RelayPeerAddr + " contents " + smsg.Text)
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
