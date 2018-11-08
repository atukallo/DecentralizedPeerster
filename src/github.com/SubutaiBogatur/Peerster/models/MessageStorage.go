package models

import (
	log "github.com/sirupsen/logrus"
	"strconv"
	"sync"
)

// struct is thread-safe, because it uses hard synchronization
type MessageStorage struct {
	// invariant: VectorClock[name] = len(RumorMessages[name])
	VectorClock                map[string]uint32          // stores nextId value
	RumorMessages              map[string][]*RumorMessage // string -> (array of RumorMessages, where array index is ID)
	NonEmptyMessagesChronOrder []*RumorMessage            // all the non-rumor-routing msgs in chronological order to display in frontend
	PrivateMessages            []*PrivateMessage          // invariant: destination = this gossiper

	mux sync.Mutex
}

// numeration from 1
func (ms *MessageStorage) GetNextMessageId(name string) uint32 {
	ms.mux.Lock()
	defer ms.mux.Unlock()

	return ms.VectorClock[name] + 1 // numeration from 1, (value or 0) + 1
}

func (ms *MessageStorage) GetCurrentStatusPacket() *StatusPacket {
	ms.mux.Lock()
	defer ms.mux.Unlock()

	want := make([]PeerStatus, 0)
	for name, nextId := range ms.VectorClock {
		currentNextId := nextId + 1 // numeration from 1
		want = append(want, PeerStatus{Identifier: name, NextID: currentNextId})
	}
	return &StatusPacket{Want: want}
}

func (ms *MessageStorage) GetRumorMessagesCopy() *[]RumorMessage {
	ms.mux.Lock()
	defer ms.mux.Unlock()

	copySlice := make([]RumorMessage, len(ms.NonEmptyMessagesChronOrder))
	for i, rmsg := range ms.NonEmptyMessagesChronOrder {
		copySlice[i] = *rmsg
	}

	return &copySlice
}

func (ms *MessageStorage) GetPrivateMessagesCopy() *[]PrivateMessage {
	ms.mux.Lock()
	defer ms.mux.Unlock()

	copySlice := make([]PrivateMessage, len(ms.PrivateMessages))
	for i, pmsg := range ms.PrivateMessages {
		copySlice[i] = *pmsg
	}

	return &copySlice
}

// counts Diff of current VectorClock with VectorClock received from peer
// returns (rmsg this peer has and another peer doesn't, does other peer has something new for this peer)
// if return[0] != nil, return[1] is not guaranteed
func (ms *MessageStorage) Diff(sp *StatusPacket) (*RumorMessage, bool) {
	ms.mux.Lock()
	defer ms.mux.Unlock()

	otherHasThisDoesnt := false

	othersMap := make(map[string]uint32)

	for _, peerStatus := range sp.Want {
		nextId := peerStatus.NextID
		nextId = peerStatus.NextID - 1 // numeration from 1
		name := peerStatus.Identifier

		if ms.VectorClock[name] > nextId {
			// we have something, the other peer doesn't
			return ms.RumorMessages[name][nextId], otherHasThisDoesnt
		} else if ms.VectorClock[name] < nextId {
			// other peer has something, we don't
			otherHasThisDoesnt = true
		}

		othersMap[name] = nextId
	}

	for name, nextId := range ms.VectorClock {
		if nextId > othersMap[name] {
			// we have something, the other peer doesn't
			return ms.RumorMessages[name][othersMap[name]], otherHasThisDoesnt
		} else if nextId < othersMap[name] {
			// other peer has something, we don't
			otherHasThisDoesnt = true // does not seem possible really
		}
	}

	return nil, otherHasThisDoesnt
}

func (ms *MessageStorage) IsNewMessage(rmsg *RumorMessage) bool {
	ms.mux.Lock()
	defer ms.mux.Unlock()

	rmsgId := rmsg.ID - 1 // numeration from 1
	origin := rmsg.OriginalName

	if rmsgId < ms.VectorClock[origin] {
		return false
	} else if rmsgId == ms.VectorClock[origin] {
		return true
	} else {
		return false // not good
	}

}

func (ms *MessageStorage) AddRumorMessage(rmsg *RumorMessage) bool {
	ms.mux.Lock()
	defer ms.mux.Unlock()

	origin := rmsg.OriginalName
	if origin == "" {
		log.Warn("got sender with empty origin!")
	}

	if ms.RumorMessages[origin] == nil {
		ms.RumorMessages[origin] = make([]*RumorMessage, 0)
	}

	rmsgId := rmsg.ID - 1 // numeration from 1
	if rmsgId < ms.VectorClock[origin] {
		return false // not new
	} else if rmsgId == ms.VectorClock[origin] {
		ms.RumorMessages[origin] = append(ms.RumorMessages[origin], rmsg)
		if rmsg.Text != "" {
			// if not rumor-routing
			ms.NonEmptyMessagesChronOrder = append(ms.NonEmptyMessagesChronOrder, rmsg)
		}
		ms.VectorClock[origin]++
	} else {
		log.Warn("messages from " + origin + " arrive not in chronological order!")
		log.Warn("got message with ID " + strconv.Itoa(int(rmsgId)) + " when value in vector clock is: " + strconv.Itoa(int(ms.VectorClock[origin])))

		if false {
			// fill missing messages with zero text
			for i := ms.VectorClock[origin]; i < rmsgId; i++ {
				brokenRmsg := &RumorMessage{ID: i, OriginalName: origin, Text: "error - chronological order broken - error"}
				ms.RumorMessages[origin] = append(ms.RumorMessages[origin], brokenRmsg)
				ms.NonEmptyMessagesChronOrder = append(ms.NonEmptyMessagesChronOrder, brokenRmsg)
				ms.VectorClock[origin]++
			}

			ms.RumorMessages[origin] = append(ms.RumorMessages[origin], rmsg)
			ms.VectorClock[origin]++
		} else {
			return false // messages arrived not in chronological order, so we will not store them, but will wait for chronological order
		}
	}

	// should add personal info to logger
	log.Debug("Vector clock is:", ms.VectorClock)

	return true // was new
}

func (ms *MessageStorage) AddPrivateMessage(pmsg *PrivateMessage, currentGossiperName string) bool {
	ms.mux.Lock()
	defer ms.mux.Unlock()

	if pmsg.Destination != currentGossiperName {
		log.Warn("want to add odd message to db")
		return false
	}

	ms.PrivateMessages = append(ms.PrivateMessages, pmsg)
	return true
}
