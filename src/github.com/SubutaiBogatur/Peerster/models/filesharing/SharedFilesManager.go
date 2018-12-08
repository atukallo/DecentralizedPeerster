package filesharing

import (
	"encoding/hex"
	"fmt"
	. "github.com/SubutaiBogatur/Peerster/config"
	. "github.com/SubutaiBogatur/Peerster/models"
	. "github.com/SubutaiBogatur/Peerster/utils"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// accessed only from message-processor thread
type SharedFilesManager struct {
	sharedFiles map[[32]byte]*sharedFile

	mux sync.Mutex

	l *log.Entry // logger
}

func InitSharedFilesManager(l *log.Entry) *SharedFilesManager {
	sfm := &SharedFilesManager{sharedFiles: make(map[[32]byte]*sharedFile), l: l}

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
// returns (Name, MetafileHash, Size)
func (sfm *SharedFilesManager) ShareFile(path string) (*string, *[32]byte, *int) {
	sfm.mux.Lock()
	defer sfm.mux.Unlock()

	path = filepath.Join(SharedFilesPath, path)
	for _, v := range sfm.sharedFiles {
		if v.Name == filepath.Base(path) {
			sfm.l.Error("such file was already shared")
		}
	}

	sf := shareFile(path)
	if sf == nil {
		sfm.l.Error("unable to share file")
		return nil, nil, nil
	}

	if _, ok := sfm.sharedFiles[sf.MetaHash]; ok {
		sfm.l.Error("such hash is already present in map!!")
		return nil, nil, nil
	}

	sfm.sharedFiles[sf.MetaHash] = sf
	fmt.Println("SHARED FILE " + sf.Name + " GOT METAHASH " + hex.EncodeToString(sf.MetaHash[:]))
	return &sf.Name, &sf.MetaHash, &sf.Size
}

func (sfm *SharedFilesManager) GetChunkOrMetafile(hashValue []byte) []byte {
	sfm.mux.Lock()
	defer sfm.mux.Unlock()

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

	sfm.l.Info("the chunk or metafile with requested hash not found in shared files")
	return nil
}

func (sfm *SharedFilesManager) GetSearchResults(keywords []string) []*SearchResult {
	sfm.mux.Lock()
	defer sfm.mux.Unlock()

	searchResults := make([]*SearchResult, 0)

	for _, sf := range sfm.sharedFiles {
		searchResults = append(searchResults, sf.getSearchResults(keywords)...)
	}

	return searchResults
}

func (sfm *SharedFilesManager) GetSharedFilesList() []string {
	sfm.mux.Lock()
	defer sfm.mux.Unlock()

	ret := make([]string, 0)
	for _, sf := range sfm.sharedFiles {
		ret = append(ret, sf.Name + " - " + hex.EncodeToString(sf.MetaHash[:]))
	}

	sort.Strings(ret)
	return ret
}
