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
	Rumor      *ClientRumorMessage
	RouteRumor *ClientRouteRumorMessage
	Private    *ClientPrivateMessage
}

type ClientRumorMessage struct {
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

func (cmsg *ClientMessage) Print() bool {
	if cmsg.Rumor != nil {
		rcmsg := cmsg.Rumor
		fmt.Println("CLIENT MESSAGE " + rcmsg.Text)
	} else if cmsg.Private != nil {
		pcmsg := cmsg.Private
		fmt.Println("CLIENT PRIVATE TO " + pcmsg.Destination + ": " + pcmsg.Text)
	} else {
		// client route rumor message
		return false
	}
	return true
}

func (agp *AddressedGossipPacket) Print() bool {
	gp := agp.Packet

	if gp.Rumor != nil {
		if gp.Rumor.Text == "" {
			return false
		}
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
	} else if gp.Private != nil {
		pmsg := gp.Private
		fmt.Println("PRIVATE origin " + pmsg.Origin + " from " + agp.Address.String() + " destination " + pmsg.Destination + " contents " + pmsg.Text)
	} else {
		return false // never should happen
	}
	return true
}
