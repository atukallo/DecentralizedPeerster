package filesharing

import (
	"github.com/SubutaiBogatur/Peerster/models"
	. "github.com/SubutaiBogatur/Peerster/utils"
	log "github.com/sirupsen/logrus"
	"os"
	"sync"
)

// unfortunately uses hard-synchronization
// struct plays double role:
// * downloaded chunks are stored here, so when the file is fully downloaded, it is rebuild from chunks and saved to (hdd|ssd)
// * we consider, that all the downloaded chunks are at the same time shared, So struct also provides access to chunks, even when the file was fully downloaded
type DownloadingFilesManager struct {
	downloadingFiles map[string]*DownloadingFile   // origin -> df
	downloadedFiles  map[[32]byte]*DownloadingFile // metahash -> df
	m                sync.Mutex
}

func InitDownloadingFilesManager() *DownloadingFilesManager {
	// clear tmp territory
	if _, err := os.Stat(DownloadsChunksPath); !os.IsNotExist(err) {
		os.RemoveAll(DownloadsChunksPath)
	}
	if _, err := os.Stat(DownloadsPath); os.IsNotExist(err) {
		os.Mkdir(DownloadsPath, FileCommonMode)
	}

	os.Mkdir(DownloadsChunksPath, FileCommonMode)

	return &DownloadingFilesManager{downloadingFiles: make(map[string]*DownloadingFile), downloadedFiles: make(map[[32]byte]*DownloadingFile)}
}

// kind of cas, returns true if really started downloading
func (dfm *DownloadingFilesManager) StartDownloadingFromOrigin(origin string, fileName string, metahash [32]byte) bool {
	// updates data in map atomically
	dfm.m.Lock()
	defer dfm.m.Unlock()

	if _, ok := dfm.downloadingFiles[origin]; ok {
		log.Info("already downloading from this guy")
		return false
	}

	df := InitDownloadingFile(fileName, metahash)
	dfm.downloadingFiles[origin] = df

	return true
}

// returns if downloading is finished, nil stays for error
func (dfm *DownloadingFilesManager) ProcessDataReply(origin string, drmsg *models.DataReply) *bool {
	dfm.m.Lock()
	defer dfm.m.Unlock()

	df, ok := dfm.downloadingFiles[origin]
	if !ok {
		log.Error("such origin is not present..")
		return nil // downloading has not really started...
	}

	isFinished := df.ProcessDataReply(drmsg)
	if isFinished != nil && *isFinished {
		downloadedFile := dfm.downloadingFiles[origin]
		delete(dfm.downloadingFiles, origin)
		dfm.downloadedFiles[downloadedFile.MetaHash] = downloadedFile // save file for chunk accessing
	}

	return isFinished
}

func (dfm *DownloadingFilesManager) GetDataRequestHash(origin string) []byte {
	dfm.m.Lock()
	defer dfm.m.Unlock()

	if _, ok := dfm.downloadingFiles[origin]; !ok {
		log.Error("doesn't have such origin")
		return nil
	}

	return dfm.downloadingFiles[origin].getDataRequest()
}

func (dfm *DownloadingFilesManager) DropDownloading(origin string) {
	dfm.m.Lock()
	defer dfm.m.Unlock()

	delete(dfm.downloadingFiles, origin)
}

func (dfm *DownloadingFilesManager) GetChunkOrMetafile(hashValue []byte) []byte {
	dfm.m.Lock()
	defer dfm.m.Unlock()

	typedHashValue, err := GetTypeStrictHash(hashValue)
	if CheckErr(err) {
		return nil
	}

	// try to find a chunk with such hash. Number of df is small, so not that long
	for _, df := range dfm.downloadingFiles {
		if df.fileHasDownloadedChunk(typedHashValue) {
			return df.getChunkOrMetafile(typedHashValue)
		}
	}

	for _, df := range dfm.downloadedFiles {
		if df.fileHasDownloadedChunk(typedHashValue) {
			return df.getChunkOrMetafile(typedHashValue)
		}
	}

	return nil
}
