package filesharing

import (
	"crypto/sha256"
	"encoding/hex"
	. "github.com/SubutaiBogatur/Peerster/utils"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"path/filepath"
)

type sharedFile struct {
	// chunks by itself are stored in _SharedFiles/{Name}/{Hash as hex string}.chunk

	Name string
	Size int // in bytes, not bigger then 2 * 1024 * 1024

	MetaHash  [32]byte
	MetaSlice []byte            // stores merged hashes of chunks in right order
	MetaSet   map[[32]byte]bool // stores hashes of chunks
}

func shareFile(path string) *sharedFile {
	path, err := filepath.Abs(path)
	if CheckErr(err) {
		return nil
	}

	if stat, err := os.Stat(path); os.IsNotExist(err) || stat.IsDir() {
		return nil
	}

	if _, err := os.Stat(SharedFilesPath); os.IsNotExist(err) {
		os.Mkdir(SharedFilesPath, FileCommonMode)
	}
	if _, err := os.Stat(SharedFilesChunksPath); os.IsNotExist(err) {
		os.Mkdir(SharedFilesChunksPath, FileCommonMode)
	}

	sharedFile := sharedFile{Name: filepath.Base(path)}

	chunksPath := filepath.Join(SharedFilesChunksPath, sharedFile.Name)
	if _, err := os.Stat(chunksPath); !os.IsNotExist(err) {
		return nil // it seems like this sharedFile is already being shared
	}

	os.Mkdir(chunksPath, FileCommonMode)

	// time to read the file and create chunks
	fileBytes, err := ioutil.ReadFile(path)
	if CheckErr(err) {
		return nil
	}

	sharedFile.Size = len(fileBytes)
	if sharedFile.Size > MaxFileSize {
		return nil
	}

	// split in chunks
	chunks := make([][]byte, 0, len(fileBytes)/FileChunkSize+1)
	var curChunk = make([]byte, 0, FileChunkSize)
	for i := 0; i < len(fileBytes); i++ {
		if i%FileChunkSize == 0 && len(curChunk) != 0 {
			chunks = append(chunks, curChunk)
			curChunk = make([]byte, 0, FileChunkSize)
		}
		curChunk = append(curChunk, fileBytes[i])
	}
	if len(curChunk) != 0 {
		chunks = append(chunks, curChunk)
	}

	sharedFile.MetaSlice = make([]byte, 0, len(chunks)*32)
	sharedFile.MetaSet = make(map[[32]byte]bool)
	for _, chunk := range chunks {
		chunkHash := sha256.Sum256(chunk)
		sharedFile.MetaSlice = append(sharedFile.MetaSlice, chunkHash[:]...)
		sharedFile.MetaSet[chunkHash] = true
		chunkFileName := hex.EncodeToString(chunkHash[:])
		ioutil.WriteFile(filepath.Join(chunksPath, chunkFileName), chunk, FileCommonMode)
		// now chunk won't be stored in ram
	}

	sharedFile.MetaHash = sha256.Sum256(sharedFile.MetaSlice)

	return &sharedFile
}

func (sf *sharedFile) chunkExists(chunkHash [32]byte) bool {
	_, ok := sf.MetaSet[chunkHash]
	return ok
}

func (sf *sharedFile) getChunk(chunkHash [32]byte) []byte {
	if !sf.chunkExists(chunkHash) {
		return nil
	}

	chunkFileName := hex.EncodeToString(chunkHash[:])
	chunkPath := filepath.Join(SharedFilesChunksPath, sf.Name, chunkFileName)
	if _, err := os.Stat(chunkPath); os.IsNotExist(err) {
		log.Error("existing chunk cannot be found!!!")
		return nil
	}

	chunkBytes, err := ioutil.ReadFile(chunkPath)
	if CheckErr(err) {
		return nil
	}

	return chunkBytes
}
