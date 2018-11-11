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

	// always send first route-rumor
	SendRouteRumorMessageToLocalPort(gossiper.GetClientAddress().Port, logger)
	if rtimer <= 0 {
		return // timer disabled
	}

	for {
		ticker := time.NewTicker(time.Duration(rtimer) * time.Second)
		<-ticker.C // wait for the timer to shoot

		SendRouteRumorMessageToLocalPort(gossiper.GetClientAddress().Port, logger)
	}
}
