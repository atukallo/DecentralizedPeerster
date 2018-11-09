package main

import (
	"flag"
	"fmt"
	. "github.com/SubutaiBogatur/Peerster/utils/send-utils"
	log "github.com/sirupsen/logrus"
)

// command line arguments
var (
	UIPort = flag.Int("UIPort", 4848, "Port, where gossiper is listening for a client. Gossiper is listening on 127.0.0.1:{port}")
	msg    = flag.String("msg", "tmptmp", "Message to send to gossiper")
	dest   = flag.String("dest", "", "Specify to send private message")
	file   = flag.String("file", "", "File name in ../_SharedFiles directory")

	logger = log.WithField("bin", "clt")
)

func main() {
	log.SetLevel(log.DebugLevel)

	flag.Parse()

	if *dest != "" && *msg != "" {
		SendPrivateMessageToLocalPort(*msg, *dest, *UIPort, logger)
	} else if *msg != "" {
		SendRumorMessageToLocalPort(*msg, *UIPort, logger)
	} else {
		logger.Warn("want to send manually rumor routing message, hmmm, not allowed...")
	}

	logger.Info("work done, shutting down")
	fmt.Println("client finished")
}
