package merkletree

import (
	"crypto/sha256"
	. "github.com/SubutaiBogatur/Peerster/config"
	. "github.com/SubutaiBogatur/Peerster/utils"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"path/filepath"
)

type merkleNode struct {
	Height    uint32
	HashValue [32]byte

	Children []*merkleNode // |children| <= chunk_size / 32
}

func constructLeafMerkleNode(hashValue [32]byte) *merkleNode {
	return &merkleNode{HashValue: hashValue, Children: nil, Height: 0}
}

func constructInnerMerkleNode(children []*merkleNode, chunksPath string) *merkleNode {
	if children == nil || len(children) == 0 {
		log.Error("empty children passed")
		return nil
	}

	height := children[0].Height
	data := make([]byte, 0, FileChunkSize)
	for _, c := range children {
		data = append(data, c.HashValue[:]...)
		if c.Height != height {
			log.Error("children vary in height!")
			return nil
		}
	}

	if len(data) > FileChunkSize {
		log.Error("got very strange file node!")
		return nil
	}

	hashValue := sha256.Sum256(data)
	ioutil.WriteFile(filepath.Join(chunksPath, GetMerkleChunkFileName(hashValue)), data, FileCommonMode)

	return &merkleNode{Height:height + 1, HashValue:hashValue, Children:children}
}
