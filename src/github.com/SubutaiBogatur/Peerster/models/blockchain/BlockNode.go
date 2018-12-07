package blockchain

import (
	"encoding/hex"
	. "github.com/SubutaiBogatur/Peerster/models"
)

type blockNode struct {
	depth  int
	parent *blockNode
	txSet  *TransactionsSet // just for O(1) access instead of linear

	block *Block
}

func InitBlockNode(parent *blockNode, block *Block) *blockNode {
	return &blockNode{parent: parent, depth: parent.depth + 1, block: block, txSet: InitFromSlice(block.Transactions)}
}

// father of fathers
func InitFakeBlockNode() *blockNode {
	return &blockNode{parent: nil, depth: 0, block: nil, txSet: nil}
}

func (bn *blockNode) isFake() bool {
	return bn.depth == 0 && bn.parent == nil
}

func (bn *blockNode) containsTransaction(tx *TxPublish) bool {
	return bn.txSet.contains(tx)
}

func (bn *blockNode) getBlockHash() [32]byte {
	if bn.isFake() {
		return [32]byte{} // crunch
	}
	return bn.block.Hash()
}

func (bn *blockNode) String() string {
	h := bn.getBlockHash()
	return hex.EncodeToString(h[:])
}

// returns correct answer only if answer exists, else not guaranteed, but not fails
func getLSA(a *blockNode, b *blockNode) *blockNode {
	if b.depth > a.depth {
		tmp := b
		b = a
		a = tmp
	}

	// b always has <= depth

	for a != nil && a.depth != b.depth {
		a = a.parent
	}

	for a != nil && b != nil && a != b {
		a = a.parent
		b = b.parent
	}

	return a
}
