package filesharing

import (
	"crypto/sha256"
	"encoding/hex"
	. "github.com/SubutaiBogatur/Peerster/models"
	. "github.com/SubutaiBogatur/Peerster/utils"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"path/filepath"
)

type DownloadingFile struct {
	Name string

	MetaHash          [32]byte
	MetaSliceByChunks [][32]byte

	ChunksToDownload map[[32]byte]bool // chunk_hash -> bool, is like a set
}

func InitDownloadingFile(name string) *DownloadingFile {
	return &DownloadingFile{Name: name, MetaSliceByChunks: nil, ChunksToDownload: nil}
}

func (df *DownloadingFile) ProcessDataReply(drpmsg *DataReply) {
	hashValue := GetTypeStrictHash(drpmsg.HashValue)
	if hashValue == nil {
		return
	}

	typedHashValue := *hashValue
	data := drpmsg.Data

	if sha256.Sum256(data) != typedHashValue {
		log.Error("got error hash-value pair!")
		return
	}

	if df.ChunksToDownload == nil {
		// this means this is the fst msg we receive
		df.MetaHash = typedHashValue
		if len(data)%32 != 0 {
			log.Error("we should have received metahash, but data seems not valid..")
			return
		}

		df.MetaSliceByChunks = make([][32]byte, 0, len(data)/32)
		df.ChunksToDownload = make(map[[32]byte]bool)
		var currentChunk [32]byte
		for i := 0; i < len(data); i++ {
			currentChunk[i%32] = data[i]
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
		if _, err := os.Stat(filepath.Join(DownloadsChunksPath, df.Name)); !os.IsNotExist(err) {
			return // seems already downloaded...
		}
		os.Mkdir(filepath.Join(DownloadsChunksPath, df.Name), FileCommonMode)

		return
	}

	// else this is not metahash:

	if _, ok := df.ChunksToDownload[typedHashValue]; !ok {
		log.Warn("got the chunk, which is not in metafile")
		return
	}

	// we got the chunk we were waiting for:
	delete(df.ChunksToDownload, typedHashValue)
	chunkFileName := hex.EncodeToString(typedHashValue[:])
	ioutil.WriteFile(filepath.Join(DownloadsChunksPath, df.Name, chunkFileName), data, FileCommonMode)

	if len(df.ChunksToDownload) != 0 {
		return
	}

	log.Info("file " + df.Name + " is downloaded, composing it..")
	fileBytes := make([]byte, 0, len(df.MetaSliceByChunks)*32)
	for _, chunkHash := range df.MetaSliceByChunks {
		chunkFileName = hex.EncodeToString(chunkHash[:])
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

	log.Info("file composed and written to persistent memory")
	if _, err := os.Stat(filepath.Join(DownloadsChunksPath, df.Name)); !os.IsNotExist(err) {
		log.Warn("such file alreaqy exists, deleting it, sorry..")
		os.Remove(filepath.Join(DownloadsChunksPath, df.Name))
	}
	ioutil.WriteFile(filepath.Join(DownloadsChunksPath, df.Name), fileBytes, FileCommonMode)
}

func (df *DownloadingFile) GetDataRequest() []byte {
	for k := range df.ChunksToDownload {
		return k[:]
	}

	// everything is downloaded already
	return nil
}
