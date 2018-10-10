package models

import (
	log "github.com/sirupsen/logrus"
	"strconv"
)

type MessageStorage struct {
	// invariant: VectorClock[name] = len(Messages[name])
	VectorClock map[string]uint32          // stores nextId value
	Messages    map[string][]*RumorMessage // string -> (array of RumorMessages, where array index is ID)
}

func (ms *MessageStorage) GetNextMessageId(name string) uint32 {
	return ms.VectorClock[name] // 0 if not found or value
}

func (ms *MessageStorage) ProcessRumorMessage(rmsg *RumorMessage) bool {
	name := rmsg.OriginalName

	if ms.Messages[name] == nil {
		ms.Messages[name] = make([]*RumorMessage, 10)
	}

	if rmsg.ID < ms.VectorClock[name] {
		return false // not new
	} else if rmsg.ID == ms.VectorClock[rmsg.OriginalName] {
		ms.Messages[name] = append(ms.Messages[name], rmsg)
		ms.VectorClock[name]++
	} else {
		log.Warn("messages from " + name + " arrive not in chronological order!")
		log.Warn("got message with ID " + strconv.Itoa(int(rmsg.ID)) + " when value in vector clock is: " + strconv.Itoa(int(ms.VectorClock[name])))

		// fill missing messages with zero text
		for i := ms.VectorClock[name]; i < rmsg.ID; i++ {
			ms.Messages[name] = append(ms.Messages[name], &RumorMessage{ID: i, OriginalName: name, Text: "error - chronological order broken - error"})
			ms.VectorClock[name]++
		}

		ms.Messages[name] = append(ms.Messages[name], rmsg)
		ms.VectorClock[name]++
	}

	// should add personal info to logger
	log.Debug("Vector clock is:")
	log.Debug(ms.VectorClock)

	return true // was new
}
