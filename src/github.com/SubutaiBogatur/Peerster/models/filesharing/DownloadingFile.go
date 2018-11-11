package filesharing

import (
	"crypto/sha256"
	"fmt"
	. "github.com/SubutaiBogatur/Peerster/models"
	. "github.com/SubutaiBogatur/Peerster/utils"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
)

type DownloadingFile struct {
	Name string

	MetaHash          [32]byte
	MetaSliceByChunks [][32]byte

	ChunksToDownload map[[32]byte]bool // chunk_hash -> bool, is like a set
}

func InitDownloadingFile(name string, metahash [32]byte) *DownloadingFile {
	return &DownloadingFile{Name: name, MetaHash: metahash} // all others nil
}

// returns true if downloading is finished
func (df *DownloadingFile) ProcessDataReply(drpmsg *DataReply) bool {
	typedHashValue, err := GetTypeStrictHash(drpmsg.HashValue)
	if CheckErr(err) {
		return false
	}

	data := drpmsg.Data
	if sha256.Sum256(data) != typedHashValue {
		log.Error("got error hash-value pair!")
		return false
	}

	// if we received metafile:
	if df.ChunksToDownload == nil {
		df.gotMetafile(typedHashValue, data)
		fmt.Println("DOWNLOADING metafile of " + df.Name + " from " + drpmsg.Origin)
		return false
	}

	// else this is not metahash:

	if _, ok := df.ChunksToDownload[typedHashValue]; !ok {
		log.Warn("got the chunk, which is not in metafile")
		return false
	}

	// we got the chunk we were waiting for:
	delete(df.ChunksToDownload, typedHashValue)
	fmt.Println("DOWNLOADING " + df.Name + " chunk " + strconv.Itoa(len(df.MetaSliceByChunks)-len(df.ChunksToDownload)) + " from " + drpmsg.Origin)
	chunkFileName := GetChunkFileName(typedHashValue)
	ioutil.WriteFile(filepath.Join(DownloadsChunksPath, df.Name, chunkFileName), data, FileCommonMode)

	if len(df.ChunksToDownload) == 0 {
		df.finishDownloading()
		return true // even if errors occured, they seem not-repairable
	}

	return false
}

func (df *DownloadingFile) gotMetafile(hashValue [32]byte, metafile []byte) {
	if len(metafile)%32 != 0 || hashValue != df.MetaHash {
		log.Error("we should have received metafile, but data seems not valid..")
		return
	}

	df.MetaSliceByChunks = make([][32]byte, 0, len(metafile)/32)
	df.ChunksToDownload = make(map[[32]byte]bool)
	var currentChunk [32]byte
	for i := 0; i < len(metafile); i++ {
		currentChunk[i%32] = metafile[i]
		if (i+1)%32 == 0 {
			df.MetaSliceByChunks = append(df.MetaSliceByChunks, currentChunk)
			df.ChunksToDownload[currentChunk] = true
		}
	}

	// clean tmp storage:
	if _, err := os.Stat(DownloadsPath); os.IsNotExist(err) {
		os.Mkdir(DownloadsPath, FileCommonMode)
	}
	if _, err := os.Stat(DownloadsChunksPath); os.IsNotExist(err) {
		os.Mkdir(DownloadsChunksPath, FileCommonMode)
	}
	fileChunksPath := filepath.Join(DownloadsChunksPath, df.Name)
	if _, err := os.Stat(fileChunksPath); !os.IsNotExist(err) {
		log.Warn("received metafile for file, which downloading is currently in progress..")
		return
	}

	os.Mkdir(fileChunksPath, FileCommonMode)
}

func (df *DownloadingFile) finishDownloading() {
	log.Info("file " + df.Name + " is downloaded, composing it..")
	fileBytes := make([]byte, 0, len(df.MetaSliceByChunks)*FileChunkSize)
	for _, chunkHash := range df.MetaSliceByChunks {
		chunkFileName := GetChunkFileName(chunkHash)
		chunkPath := filepath.Join(DownloadsChunksPath, df.Name, chunkFileName)
		if _, err := os.Stat(chunkPath); os.IsNotExist(err) {
			log.Error("existing chunk cannot be found!!!")
			return
		}

		chunkBytes, err := ioutil.ReadFile(chunkPath)
		if CheckErr(err) {
			return
		}

		fileBytes = append(fileBytes, chunkBytes...)
	}

	log.Info("file composed and being written to persistent memory")
	if _, err := os.Stat(filepath.Join(DownloadsPath, df.Name)); !os.IsNotExist(err) {
		log.Warn("such file alreaqy exists in downloads dir, deleting old file, sorry..")
		os.Remove(filepath.Join(DownloadsPath, df.Name))
	}
	ioutil.WriteFile(filepath.Join(DownloadsPath, df.Name), fileBytes, FileCommonMode)

	log.Debug("cleaning the temporary storage..")
	os.RemoveAll(filepath.Join(DownloadsChunksPath, df.Name))
}

func (df *DownloadingFile) getDataRequest() []byte {
	for k := range df.ChunksToDownload {
		return k[:]
	}

	// everything is downloaded already
	return nil
}
