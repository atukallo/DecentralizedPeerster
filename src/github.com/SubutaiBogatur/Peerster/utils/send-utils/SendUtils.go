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
	logDebug("sending route-rumor msg to local client port", logger)
	rrcmsg := &ClientRouteRumorMessage{}
	cmsg := &ClientMessage{RouteRumor: rrcmsg}
	sendMessageToLocalPort(cmsg, port, logger)
}

func SendRumorMessageToLocalPort(message string, port int, logger *log.Entry) {
	logDebug("sending (not-route) rumor msg to local client port", logger)
	rcmsg := &ClientRumorMessage{Text: message}
	cmsg := &ClientMessage{Rumor: rcmsg}
	sendMessageToLocalPort(cmsg, port, logger)
}

func SendPrivateMessageToLocalPort(message string, destination string, port int, logger *log.Entry) {
	logDebug("sending private msg to local client port", logger)
	pcmsg := &ClientPrivateMessage{Text: message, Destination: destination}
	cmsg := &ClientMessage{Private: pcmsg}
	sendMessageToLocalPort(cmsg, port, logger)
}

func SendToShareMessageToLocalPort(path string, port int, logger *log.Entry) {
	logDebug("sending to-share msg to local client port", logger)
	tsmsg := &ClientToShareMessage{Path: path}
	csmsg := &ClientMessage{ToShare: tsmsg}
	sendMessageToLocalPort(csmsg, port, logger)
}

func SendToDownloadMessageToLocalPort(name string, hashString string, destination string, port int, logger *log.Entry) {
	logDebug("sending to-download msg to local client port", logger)
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

func SendToSearchMessaageToLocalPort(keywords []string, budget uint64, port int, logger *log.Entry) {
	logDebug("sending to-search msg to local client port", logger)
	tsmsg := &ClientToSearchMessage{Keywords:keywords, Budget:budget, DownloadAfterSearch:true} // should start downloading from cli
	csmsg := &ClientMessage{ToSearch: tsmsg}
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
		logError("error dialing: " + err.Error(), logger)
	}

	n, err := connToGossiper.Write(packetBytes)
	if err != nil {
		logError("error when writing to connection: " + err.Error() + " n is " + strconv.Itoa(n), logger)
	}
}
