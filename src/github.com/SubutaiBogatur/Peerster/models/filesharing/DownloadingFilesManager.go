package filesharing

import (
	"github.com/SubutaiBogatur/Peerster/models"
	. "github.com/SubutaiBogatur/Peerster/utils"
	log "github.com/sirupsen/logrus"
	"os"
	"sync"
)

// every value in map is accessed only with 1 process at the same time!
type DownloadingFilesManager struct {
	downloadingFiles map[string]*DownloadingFile // origin -> df
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

	return &DownloadingFilesManager{downloadingFiles: make(map[string]*DownloadingFile)}
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

// returns if downloading is finished
func (dfm *DownloadingFilesManager) ProcessDataReply(origin string, drmsg *models.DataReply) bool {
	df, ok := dfm.downloadingFiles[origin] // reading can be done without mutex, because every entry is accessed with one thread
	if !ok {
		log.Error("such origin is not present..")
		return true // downloading is not really started...
	}

	isFinished := df.ProcessDataReply(drmsg)
	if isFinished {
		dfm.m.Lock() // map modification better be synchronized
		delete(dfm.downloadingFiles, origin)
		dfm.m.Unlock()
	}

	return isFinished
}

func (dfm *DownloadingFilesManager) GetDataRequestHash(origin string) []byte {
	dfm.m.Lock()
	defer dfm.m.Unlock()

	return dfm.downloadingFiles[origin].getDataRequest()
}

func (dfm *DownloadingFilesManager) DropDownloading(origin string) {
	dfm.m.Lock()
	defer dfm.m.Unlock()

	delete(dfm.downloadingFiles, origin)
}
