package blockchain

import (
	"encoding/hex"
	"fmt"
	. "github.com/SubutaiBogatur/Peerster/config"
	. "github.com/SubutaiBogatur/Peerster/models"
	log "github.com/sirupsen/logrus"
	"math/rand"
	"strconv"
	"sync"
	. "time"
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
	bm.blocks[bm.tail.getBlockHash()] = bm.tail // a bit dangerous, because real hash is not zeroes

	return bm
}

// returns true if new & correct block
func (bm *BlockchainManager) AddBlock(block *Block) bool {
	// 1) check if new
	// 2) check if parent is known & block is correct (Proof-of-Work=POW)
	// 3) append to parent with checking:
	// 3.1) if parent is    tail of longest chain, just append and change pending transactions
	// 3.2) if parent isn't tail of longest chain:
	// 3.2.1) if chain becomes longest after appending, rollback current history & apply new history & calculate new pendingTransactions set
	// 3.2.2) if chain remains short, do nothing
	bm.m.Lock()
	defer bm.printBlocksMap()
	defer bm.m.Unlock()

	if _, ok := bm.blocks[block.Hash()]; ok {
		bm.l.Warn("block was already received: " + block.String())
		return false
	}

	if _, ok := bm.blocks[block.PrevHash]; !ok {
		bm.l.Warn("parent of block isn't known, block: " + block.String())
		return false
	}

	if !block.IsGood() {
		bm.l.Warn("block is malicious, its hash is not good, block: "+block.String()+" ", block.Transactions, block.PrevHash, block.Nonce)
		return false
	}

	// so, is good and is new, let's append to the tree
	if block.PrevHash == bm.tail.getBlockHash() {
		bm.l.Info("block prolongs longest chain, good, block: " + block.String())
		bm.tail = InitBlockNode(bm.tail, block)
		bm.blocks[block.Hash()] = bm.tail
		bm.pendingTx.subtract(bm.tail.txSet)
		bm.printLongestChain()
		return true
	}

	parentNode := bm.blocks[block.PrevHash]
	// if chain doesn't become longest
	if parentNode.depth < bm.tail.depth {
		bm.l.Info("block prolongs short chain, block: " + block.String())
		bm.blocks[block.Hash()] = InitBlockNode(parentNode, block)
		fmt.Println("FORK-SHORTER " + block.String())
		return true
	}

	bm.l.Info("block prolongs short chain, which become longest, reapplying history..., block: " + block.String())
	// now the most interesting case
	lsa := getLSA(bm.tail, parentNode)
	// roll-back history
	blocksUndone := 0
	for curBlock := bm.tail; curBlock != lsa; curBlock = curBlock.parent {
		bm.pendingTx.union(curBlock.txSet)
		blocksUndone++
	}
	fmt.Println("FORK-LONGER rewind " + strconv.Itoa(blocksUndone) + " blocks")

	bm.tail = InitBlockNode(parentNode, block)
	bm.blocks[block.Hash()] = bm.tail
	// apply new history
	for curBlock := bm.tail; curBlock != lsa; curBlock = curBlock.parent {
		bm.pendingTx.subtract(curBlock.txSet)
	}
	bm.printLongestChain()

	return true
}

// returns true if new & correct transaction
func (bm *BlockchainManager) AddTransaction(tx *TxPublish) bool {
	// check if new & valid (ie such name not taken yet)
	bm.m.Lock()
	defer bm.m.Unlock()

	if bm.pendingTx.contains(tx) {
		bm.l.Info("transaction is already pending, dropping it")
		return false
	}

	if bm.pendingTx.containsFilename(tx.File.Name) {
		bm.l.Warn("such name was already reserved in pending transactions by file with metahash: " + hex.EncodeToString(bm.pendingTx.get(tx.File.Name).File.MetafileHash))
		return false
	}

	// go along the official history and check if transaction is new & correct
	for curBlock := bm.tail; !curBlock.isFake(); curBlock = curBlock.parent {
		if curBlock.txSet.contains(tx) {
			bm.l.Info("such transaction was already registered in block " + curBlock.String())
			return false
		}

		if curBlock.txSet.containsFilename(tx.File.Name) {
			bm.l.Warn("such name was already reserved in block " + curBlock.String() + " by file with metahash: " + hex.EncodeToString(curBlock.txSet.get(tx.File.Name).File.MetafileHash))
			return false
		}
	}

	// pending & history checked both for correctness of the transaction and also for its newness
	bm.pendingTx.add(tx)
	bm.l.Info("transaction added to pending set")
	return true

}

// is called from distinct thread
// tries to mine new block. When succeeds, adds it to the blockchain and returns (block, time cpu was actually mining)
func (bm *BlockchainManager) DoMining() (*Block, Duration) {
	failedAttempts := 0
	sleepTimes := 0
	start := Now()

	// mining is done very often, so not taking lock every time, but only when possibly good block found, when decide to finally check it
	// there is a potential danger of simultaneous iteration & editing, which may cause fail, it can be avoided with taking lock every time
	for {
		if bm.pendingTx.isEmpty() {
			sleepTimes++
			Sleep(BlockchainNoTxTimeout)
			continue
		}

		nonceSlice := make([]byte, 32)
		rand.Read(nonceSlice)

		transactionsCopy := bm.pendingTx.getSliceCopy()
		tailHash := bm.tail.getBlockHash()
		newBlock := Block{PrevHash: tailHash, Transactions: transactionsCopy}
		copy(newBlock.Nonce[:], nonceSlice) // convert slice to fixed-size array

		if !newBlock.IsGood() {
			failedAttempts++
			continue
		}

		// something interesting is happening
		bm.m.Lock()
		// check once again atomically, that everything is good
		newBlock.Transactions = bm.pendingTx.getSliceCopy()
		newBlock.PrevHash = bm.tail.getBlockHash()
		if !newBlock.IsGood() || bm.pendingTx.isEmpty() {
			bm.l.Warn("very rare thing happened: block was good before taking locks, and now is bad.. (or maybe tx is empty)")
			failedAttempts++
			bm.m.Unlock()
			continue
		}

		// block is truly good
		timeUsed := Since(start) - Duration(sleepTimes)*BlockchainNoTxTimeout
		bm.l.Info("new block mined, spent " + strconv.Itoa(failedAttempts) + " attempts and " + fmt.Sprint(timeUsed.Seconds()) + " seconds, hash is: " + newBlock.String())
		fmt.Println("FOUND-BLOCK " + newBlock.String())

		// adding new block to blockchain
		bm.tail = InitBlockNode(bm.tail, &newBlock)
		bm.blocks[newBlock.Hash()] = bm.tail
		bm.pendingTx.clear()
		bm.l.Info("added new block to blockhain, now biggest-chain-depth is: " + fmt.Sprint(bm.tail.depth))
		bm.printLongestChain()
		bm.printBlocksMap()
		bm.m.Unlock()

		return &newBlock, timeUsed
	}

	return nil, 0 // cannot happen
}

// call after tail is updated
func (bm *BlockchainManager) printLongestChain() {
	str := "CHAIN"
	for curBlock := bm.tail; !curBlock.isFake(); curBlock = curBlock.parent {
		txs := ""
		for i, tx := range curBlock.block.Transactions {
			txs += tx.File.Name
			if i != len(curBlock.block.Transactions)-1 {
				txs += ","
			}
		}

		str += " " + curBlock.String() + ":" + curBlock.parent.String() + ":" + txs
	}

	log.Debug(str)
	fmt.Println(str)
}

func (bm *BlockchainManager) printBlocksMap() {
	// for debug purposes
	str := "blocks-map\n"
	for h, b := range bm.blocks {
		str += hex.EncodeToString(h[:])
		str += " - "
		str += b.String()
		if !b.isFake() {
			str += " ("
			for i, tx := range b.block.Transactions {
				str += tx.File.Name
				if i != len(b.block.Transactions) - 1 {
					str += " "
				}
			}
			str += "; " + hex.EncodeToString(b.block.PrevHash[:])
			str += "; " + hex.EncodeToString(b.block.Nonce[:])
			str += ") "
		}

		str += "\n"
	}

	//bm.l.Debug(str)
}
