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
	Simple  *SimpleMessage
	Rumor   *RumorMessage
	Status  *StatusPacket
	Private *PrivateMessage
}

type SimpleMessage struct {
	OriginalName  string // name of original gossiper sender
	RelayPeerAddr string // address of latest peer retranslator in the form ip:port
	Text          string
}

type PrivateMessage struct {
	Origin      string
	ID          uint32
	Text        string
	Destination string
	HopLimit    uint32
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
	RumorOrSimple *ClientRumorOrSimpleMessage
	RouteRumor    *ClientRouteRumorMessage
	Private       *ClientPrivateMessage
}

type ClientRumorOrSimpleMessage struct {
	Text string
}

type ClientRouteRumorMessage struct{}

type ClientPrivateMessage struct {
	Text        string
	Destination string
}

func (rmsg *RumorMessage) String() string {
	return rmsg.OriginalName + ":" + strconv.Itoa(int(rmsg.ID))
}

func (cmsg *ClientMessage) Print() {
	if cmsg.RumorOrSimple != nil {
		cmsg.RumorOrSimple.Print()
	} else if cmsg.Private != nil {
		cmsg.Private.Print()
	}
}

func (msg *ClientRumorOrSimpleMessage) Print() {
	if msg.Text == "" {
		return
	}
	fmt.Println("CLIENT MESSAGE " + msg.Text)
}

func (pmsg *ClientPrivateMessage) Print() {
	fmt.Println("CLIENT PRIVATE MESSAGE TO " + pmsg.Destination + ": " + pmsg.Text)
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
