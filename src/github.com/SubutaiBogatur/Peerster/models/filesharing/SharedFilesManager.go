package filesharing

import (
	"encoding/hex"
	"fmt"
	. "github.com/SubutaiBogatur/Peerster/utils"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
)

// accessed only from message-processor thread
type SharedFilesManager struct {
	sharedFiles map[[32]byte]*sharedFile
}

func InitSharedFilesManager() *SharedFilesManager {
	sfm := &SharedFilesManager{sharedFiles: make(map[[32]byte]*sharedFile)}

	// when initting let's clear state and tmp files:
	if _, err := os.Stat(SharedFilesChunksPath); !os.IsNotExist(err) {
		os.RemoveAll(SharedFilesChunksPath)
	}
	if _, err := os.Stat(SharedFilesPath); os.IsNotExist(err) {
		os.Mkdir(SharedFilesPath, FileCommonMode)
	}
	os.Mkdir(SharedFilesChunksPath, FileCommonMode)

	return sfm
}

// accepts path relative to _SharedFiles directory
func (sfm *SharedFilesManager) ShareFile(path string) {
	path = filepath.Join(SharedFilesPath, path)
	for _, v := range sfm.sharedFiles {
		if v.Name == filepath.Base(path) {
			log.Error("such file was already shared")
		}
	}

	sf := shareFile(path)
	if sf == nil {
		log.Error("unable to share file")
		return
	}

	if _, ok := sfm.sharedFiles[sf.MetaHash]; ok {
		log.Error("such hash is already present in map!!")
		return
	}

	sfm.sharedFiles[sf.MetaHash] = sf
	fmt.Println("SHARED FILE " + sf.Name + " GOT METAHASH " + hex.EncodeToString(sf.MetaHash[:]))
}

func (sfm *SharedFilesManager) GetChunkOrMetafile(hashValue []byte) []byte {
	typedHashValue, err := GetTypeStrictHash(hashValue)
	if CheckErr(err) {
		return nil
	}

	if sf, ok := sfm.sharedFiles[typedHashValue]; ok {
		return sf.MetaSlice // metahash was given to function, returning metafile
	}

	// else try to find a chunk with such hash. Number of sf is small, so not that long
	for _, sf := range sfm.sharedFiles {
		if sf.chunkBelongsToFile(typedHashValue) {
			return sf.getChunk(typedHashValue)
		}
	}

	log.Warn("the chunk or metafile with requested hash not found")
	return nil
}
