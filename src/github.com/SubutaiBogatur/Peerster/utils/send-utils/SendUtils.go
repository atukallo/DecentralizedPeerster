package send_utils

import (
	"encoding/hex"
	. "github.com/SubutaiBogatur/Peerster/models"
	. "github.com/SubutaiBogatur/Peerster/utils"
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

func SendRouteRumorMessageToLocalPort(port int, logger *log.Entry) {
	rrcmsg := &ClientRouteRumorMessage{}
	cmsg := &ClientMessage{RouteRumor: rrcmsg}
	sendMessageToLocalPort(cmsg, port, logger)
}

func SendRumorMessageToLocalPort(message string, port int, logger *log.Entry) {
	rcmsg := &ClientRumorMessage{Text: message}
	cmsg := &ClientMessage{Rumor: rcmsg}
	sendMessageToLocalPort(cmsg, port, logger)
}

func SendPrivateMessageToLocalPort(message string, destination string, port int, logger *log.Entry) {
	pcmsg := &ClientPrivateMessage{Text: message, Destination: destination}
	cmsg := &ClientMessage{Private: pcmsg}
	sendMessageToLocalPort(cmsg, port, logger)
}

func SendToShareMessageToLocalPort(path string, port int, logger *log.Entry) {
	tsmsg := &ClientToShareMessage{Path: path}
	csmsg := &ClientMessage{ToShare: tsmsg}
	sendMessageToLocalPort(csmsg, port, logger)
}

func SendToDownloadMessageToLocalPort(name string, hashString string, destination string, port int, logger *log.Entry) {
	hashValue, err := hex.DecodeString(hashString)
	if CheckErr(err) {
		return
	}
	typedHashValue, err := GetTypeStrictHash(hashValue)
	if CheckErr(err) {
		return
	}

	tsmsg := &ClientToDownloadMessage{Name: name, Destination: destination, HashValue: typedHashValue}
	csmsg := &ClientMessage{ToDownload: tsmsg}
	sendMessageToLocalPort(csmsg, port, logger)
}

func sendMessageToLocalPort(cmsg *ClientMessage, port int, logger *log.Entry) {
	packetBytes, err := protobuf.Encode(cmsg)
	if err != nil {
		logError("unable to send msg: "+err.Error(), logger)
		return
	}

	gossiperAddr, err := ResolveUDPAddr("udp4", LocalIp+":"+strconv.Itoa(port))
	if err != nil {
		logError("unable to send msg: "+err.Error(), logger)
		return
	}

	logInfo("sending message to "+gossiperAddr.String(), logger)

	connToGossiper, err := Dial("udp4", gossiperAddr.String())
	if err != nil {
		logger.Error("error dialing: " + err.Error())
	}

	n, err := connToGossiper.Write(packetBytes)
	if err != nil {
		logger.Error("error when writing to connection: " + err.Error() + " n is " + strconv.Itoa(n))
	}
}
