package models

import (
	log "github.com/sirupsen/logrus"
	"strconv"
	"sync"
)

// struct is thread-safe, because it uses hard synchronization
type MessageStorage struct {
	// invariant: VectorClock[name] = len(Messages[name])
	VectorClock        map[string]uint32          // stores nextId value
	Messages           map[string][]*RumorMessage // string -> (array of RumorMessages, where array index is ID)
	MessagesChronOrder []*RumorMessage

	mux sync.Mutex
}

// numeration from 1
func (ms *MessageStorage) GetNextMessageId(name string) uint32 {
	ms.mux.Lock()
	defer ms.mux.Unlock()

	return ms.VectorClock[name] + 1 // numeration from 1, value or 0 + 1
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

func (ms *MessageStorage) GetMessagesCopy() *[]RumorMessage {
	ms.mux.Lock()
	defer ms.mux.Unlock()

	copySlice := make([]RumorMessage, len(ms.MessagesChronOrder))
	for i, rmsg := range ms.MessagesChronOrder {
		copySlice[i] = *rmsg
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
			return ms.Messages[name][nextId], otherHasThisDoesnt
		} else if ms.VectorClock[name] < nextId {
			// other peer has something, we don't
			otherHasThisDoesnt = true
		}

		othersMap[name] = nextId
	}

	for name, nextId := range ms.VectorClock {
		if nextId > othersMap[name] {
			// we have something, the other peer doesn't
			return ms.Messages[name][othersMap[name]], otherHasThisDoesnt
		} else if nextId < othersMap[name] {
			// other peer has something, we don't
			otherHasThisDoesnt = true // does not seem possible really
		}
	}

	return nil, otherHasThisDoesnt
}

func (ms *MessageStorage) AddRumorMessage(rmsg *RumorMessage) bool {
	ms.mux.Lock()
	defer ms.mux.Unlock()

	name := rmsg.OriginalName
	if name == "" {
		log.Warn("got sender with empty name!")
	}

	if ms.Messages[name] == nil {
		ms.Messages[name] = make([]*RumorMessage, 0)
	}

	rmsgId := rmsg.ID - 1 // numeration from 1
	if rmsgId < ms.VectorClock[name] {
		return false // not new
	} else if rmsgId == ms.VectorClock[rmsg.OriginalName] {
		ms.Messages[name] = append(ms.Messages[name], rmsg)
		ms.MessagesChronOrder = append(ms.MessagesChronOrder, rmsg)
		ms.VectorClock[name]++
	} else {
		log.Warn("messages from " + name + " arrive not in chronological order!")
		log.Warn("got message with ID " + strconv.Itoa(int(rmsgId)) + " when value in vector clock is: " + strconv.Itoa(int(ms.VectorClock[name])))

		if false {
			// fill missing messages with zero text
			for i := ms.VectorClock[name]; i < rmsgId; i++ {
				brokenRmsg := &RumorMessage{ID: i, OriginalName: name, Text: "error - chronological order broken - error"}
				ms.Messages[name] = append(ms.Messages[name], brokenRmsg)
				ms.MessagesChronOrder = append(ms.MessagesChronOrder, brokenRmsg)
				ms.VectorClock[name]++
			}

			ms.Messages[name] = append(ms.Messages[name], rmsg)
			ms.VectorClock[name]++
		} else {
			return false // messages arrived not in chronological order, so we will not store them, but will wait for chronological order
		}
	}

	// should add personal info to logger
	log.Debug("Vector clock is:", ms.VectorClock)

	return true // was new
}
