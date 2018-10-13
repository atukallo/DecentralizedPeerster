package models

import (
	"fmt"
	"net"
	"strconv"
)

type AddressedGossipPacket struct {
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

func (rmsg *RumorMessage) String() string {
	return rmsg.OriginalName + ":" + strconv.Itoa(int(rmsg.ID))
}

func (cmsg *ClientMessage) Print() {
	fmt.Println("CLIENT MESSAGE " + cmsg.Text)
}

func (agp *AddressedGossipPacket) Print() {
	gp := agp.Packet

	if gp.Rumor != nil {
		rmsg := gp.Rumor
		fmt.Println("RUMOR origin " + rmsg.OriginalName + " from " + agp.Address.String() + " ID " + strconv.Itoa(int(rmsg.ID)) + " contents " + rmsg.Text)
	} else if gp.Status != nil {
		status := gp.Status
		fmt.Print("STATUS from " + agp.Address.String())
		for _, peerStatus := range status.Want {
			fmt.Print(" peer " + peerStatus.Identifier + " nextID " + strconv.Itoa(int(peerStatus.NextID)))
		}
		fmt.Println()
	} else if gp.Simple != nil {
		smsg := gp.Simple
		fmt.Println("SIMPLE MESSAGE origin " + smsg.OriginalName + " from " + smsg.RelayPeerAddr + " contents " + smsg.Text)
	}
}

func (gp *GossipPacket) printPackage(knownPeers []*net.UDPAddr, isFromClient bool) {
	var smsg *SimpleMessage = gp.Simple

	if isFromClient {
		fmt.Println("CLIENT MESSAGE " + smsg.Text)
	} else {
	}
}
