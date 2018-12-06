package gossiper

import (
	. "github.com/SubutaiBogatur/Peerster/utils/send-utils"
	log "github.com/sirupsen/logrus"
	"time"
)

// route-rumors thread
func StartRouteRumorsSpreading(gossiper *Gossiper, rtimer int) {
	logger := log.WithField("bin", "rr").WithField("a", gossiper.GetPeerAddress().String())
	logger.Info("started route-rumor-spreading thread")

	if rtimer <= 0 {
		logger.Info("route-rumor-spreading is actually disabled, turning it off")
		return // timer disabled
	}

	// send first route-rumor
	SendRouteRumorMessageToLocalPort(gossiper.GetClientAddress().Port, logger)

	for {
		time.Sleep(time.Duration(rtimer) * time.Second)

		SendRouteRumorMessageToLocalPort(gossiper.GetClientAddress().Port, logger)
	}
}
