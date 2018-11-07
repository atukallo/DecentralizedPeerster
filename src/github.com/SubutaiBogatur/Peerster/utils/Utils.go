package utils

import (
	. "github.com/SubutaiBogatur/Peerster/models"
	"github.com/dedis/protobuf"
	log "github.com/sirupsen/logrus"
	. "net"
	"strconv"
)

func logError(msg string, logger *log.Entry) {
	if logger == nil {
		log.Error(msg)
	} else {
		logger.Error(msg)
	}
}

func logInfo(msg string, logger *log.Entry) {
	if logger == nil {
		log.Info(msg)
	} else {
		logger.Info(msg)
	}
}

func logDebug(msg string, logger *log.Entry) {
	if logger == nil {
		log.Debug(msg)
	} else {
		logger.Debug(msg)
	}
}

func SendMessageToLocalPort(message string, port int, logger *log.Entry) {
	msg := &ClientMessage{Text:message}
	packetBytes, err := protobuf.Encode(msg)
	if err != nil {
		logError("unable to send msg: " + err.Error(), logger)
		return
	}

	gossiperAddr, err := ResolveUDPAddr("udp4", LocalIp+":"+strconv.Itoa(port))
	if err != nil {
		logError("unable to send msg: " + err.Error(), logger)
		return
	}

	logInfo("sending message to " + gossiperAddr.String(), logger)
	logDebug("msg is: " + string(message), logger)

	connToGossiper, err := Dial("udp4", gossiperAddr.String())
	if err != nil {
		logger.Error("error dialing: " + err.Error())
	}

	n, err := connToGossiper.Write(packetBytes)
	if err != nil {
		logger.Error("error when writing to connection: " + err.Error() + " n is " + strconv.Itoa(n))
	}
}