package blockchain

import (
	. "github.com/SubutaiBogatur/Peerster/models"
	log "github.com/sirupsen/logrus"
	"sync"
)

type BlockchainManager struct {
	tail      *blockNode              // tail of the longest chain in the tree
	blocks    map[[32]byte]*blockNode // block_hash -> ptr to node in the tree
	pendingTx *TransactionsSet

	m sync.Mutex
	l *log.Entry // logger
}

func InitBlockchainManager(l *log.Entry) *BlockchainManager {
	bm := &BlockchainManager{l: l, blocks: make(map[[32]byte]*blockNode), pendingTx: InitEmpty()}

	bm.tail = InitFakeBlockNode()

	var typedHashValue [32]byte // just 32 type-strict zeroes
	bm.blocks[typedHashValue] = bm.tail

	return bm
}

func (bm *BlockchainManager) addBlock(block *Block) {

}

func (bm *BlockchainManager) addTransaction(tx *TxPublish) bool {

}

func (bm *BlockchainManager) mineBlock() *Block {

}
