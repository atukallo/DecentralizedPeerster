package filesharing

import (
	. "github.com/SubutaiBogatur/Peerster/utils"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
)

type SharedFilesManager struct {
	sharedFiles map[[32]byte]*sharedFile
}

func InitSharedFilesManager() *SharedFilesManager {
	sfm := &SharedFilesManager{sharedFiles: make(map[[32]byte]*sharedFile)}

	// when initting let's clear state and tmp files:
	if _, err := os.Stat(SharedFilesChunksPath); !os.IsNotExist(err) {
		os.RemoveAll(SharedFilesChunksPath)
	}
	os.Mkdir(SharedFilesChunksPath, FileCommonMode)

	return sfm
}

// accepts path relative to _SharedFiles directory
func (sfm *SharedFilesManager) ShareFile(path string) {
	path = filepath.Join(SharedFilesPath, path)
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
}

func (sfm *SharedFilesManager) GetChunk(hashValue []byte) []byte {
	if len(hashValue) != 32 {
		log.Error("invalid hash value passed")
		return nil
	}

	var typedHashValue [32]byte
	for i, b := range hashValue {
		typedHashValue[i] = b
	}

	if sf, ok := sfm.sharedFiles[typedHashValue]; ok {
		return sf.MetaSlice // metahash was given to function, returning metafile
	}

	// else try to find a chunk with such hash, number of sf is small, so not that long
	for _, sf := range sfm.sharedFiles {
		if sf.chunkExists(typedHashValue) {
			return sf.getChunk(typedHashValue)
		}
	}

	log.Warn("the chunk / metafile with requested hash not found")
	return nil
}
