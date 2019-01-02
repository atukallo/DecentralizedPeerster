package merkletree

import (
	"crypto/sha256"
	. "github.com/SubutaiBogatur/Peerster/config"
	. "github.com/SubutaiBogatur/Peerster/utils"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

type MerkleSharedFile struct {
	// chunks by itself are stored in _SharedFiles/{Name}/{Hash as hex string}.merkle on disk

	// leaves have height 0. The tree with height k can store file with size < c^(i+1) / 32^i, where c is chunk-size
	// with chunk-size=8kb and height=3 one can share a file of 100gb size already

	Name string
	Size int64 // in bytes, read by parts during indexing, in RAM only tree is stored, ie we want filesize / chunksize * 32 < RAM iff filesize < RAM * chunksize / 32 ie almost unlimited

	RootNode *merkleNode
	NodeSet  map[[32]byte]*merkleNode // hash -> node
}

func SshareFile(path string) *MerkleSharedFile {
	path, err := filepath.Abs(path)
	if CheckErr(err) {
		return nil
	}

	if stat, err := os.Stat(path); os.IsNotExist(err) || stat.IsDir() {
		log.Error("file to share doesn't exist")
		return nil
	}

	if _, err := os.Stat(SharedFilesPath); os.IsNotExist(err) {
		os.Mkdir(SharedFilesPath, FileCommonMode)
	}
	if _, err := os.Stat(SharedFilesChunksPath); os.IsNotExist(err) {
		os.Mkdir(SharedFilesChunksPath, FileCommonMode)
	}

	sharedFile := MerkleSharedFile{Name: filepath.Base(path)}

	chunksPath := filepath.Join(SharedFilesChunksPath, sharedFile.Name)
	if _, err := os.Stat(chunksPath); !os.IsNotExist(err) {
		return nil // it seems like this sharedFile is already being shared
	}

	os.Mkdir(chunksPath, FileCommonMode)

	// tree is built level by level
	curLevel := make([]*merkleNode, 0)
	f, err := os.Open(path)
	if CheckErr(err) {
		return nil
	}

	fs, err := f.Stat()
	if CheckErr(err) {
		return nil
	}
	sharedFile.Size = fs.Size()
	nodeset := make(map[[32]byte]*merkleNode)

	// build 0th level
	offset := int64(0)
	buffer := make([]byte, FileChunkSize)
	for {
		n, err := f.ReadAt(buffer, offset)
		offset += int64(n)
		if err == io.EOF {
			break // file read succesfully
		}

		if n == 0 || CheckErr(err) {
			log.Error("error when reading a file")
			return nil
		}

		curChunk := buffer[0:n]

		chunkHash := sha256.Sum256(curChunk)
		ioutil.WriteFile(filepath.Join(chunksPath, GetMerkleChunkFileName(chunkHash)), curChunk, FileCommonMode)
		node := constructLeafMerkleNode(chunkHash)
		curLevel = append(curLevel, node)
		nodeset[chunkHash] = node
	}

	// build upper levels one-by-one
	childrenNumber := FileChunkSize / 32
	for len(curLevel) > 1 {
		newLevel := make([]*merkleNode, 0)
		curChildren := make([]*merkleNode, 0, childrenNumber)
		for i := 0; i < len(curLevel); i++ {
			curChildren = append(curChildren, curLevel[i])
			if (i+1)%childrenNumber == 0 || (i+1) == len(curLevel) {
				// children collected for a parent creation
				parentNode := constructInnerMerkleNode(curChildren, chunksPath)
				newLevel = append(newLevel, parentNode)
				nodeset[parentNode.HashValue] = parentNode

				curChildren = make([]*merkleNode, 0, childrenNumber)
			}
		}

		curLevel = newLevel
	}

	if len(curLevel) == 0 {
		log.Error("levels built wrongly")
		return nil
	}

	sharedFile.RootNode = curLevel[0]
	sharedFile.NodeSet = nodeset

	return &sharedFile
}
