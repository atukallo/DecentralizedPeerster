package utils

import (
	log "github.com/sirupsen/logrus"
)

func CheckErr(err error) bool {
	return CheckError(err, nil)
}

func CheckError(err error, logger *log.Entry) bool {
	if err != nil {
		if logger == nil {
			log.Warn("got error: " + err.Error())
		} else {
			logger.Warn("got error: " + err.Error())
		}
		return true // error detected
	}
	return false
}