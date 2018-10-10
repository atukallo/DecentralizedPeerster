package models

import "fmt"

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