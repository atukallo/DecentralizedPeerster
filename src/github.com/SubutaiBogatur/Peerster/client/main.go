package main

import (
	"flag"
	"fmt"
	. "github.com/SubutaiBogatur/Peerster/utils"
	log "github.com/sirupsen/logrus"
)

// command line arguments
var (
	UIPort     = flag.Int("UIPort", 4848, "Port, where gossiper is listening for a client. Gossiper is listening on 127.0.0.1:{port}")
	msg        = flag.String("msg", "tmptmp", "Message to send to gossiper")

	logger = log.WithField("bin", "clt")
)

func main() {
	log.SetLevel(log.DebugLevel)

	flag.Parse()

	SendMessageToLocalPort(*msg, *UIPort, logger)

	logger.Info("work done, shutting down")
	fmt.Println("client finished")
}
