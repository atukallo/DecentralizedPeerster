package utils

import (
	"encoding/hex"
	"github.com/SubutaiBogatur/Peerster/models"
	log "github.com/sirupsen/logrus"
)

// returns (err != nil)
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

func GetTypeStrictHash(hashValue []byte) ([32]byte, error) {
	var typedHashValue [32]byte
	if len(hashValue) != 32 {
		return typedHashValue, models.PeersterError{ErrorMsg:"invalid hash value passed, len is strange"}
	}

	for i, b := range hashValue {
		typedHashValue[i] = b
	}

	return typedHashValue, nil
}

func GetChunkFileName(hashValue [32]byte) string {
	return hex.EncodeToString(hashValue[:]) + ".chunk"
}