package blockchain

import . "github.com/SubutaiBogatur/Peerster/models"

// it's owful, that go doesn't have generics/templates
type TransactionsSet struct {
	s map[*TxPublish]bool
}

func InitEmpty() *TransactionsSet {
	return &TransactionsSet{s: make(map[*TxPublish]bool)}
}

func InitFromSlice(s []TxPublish) *TransactionsSet {
	ts := InitEmpty()
	for _, tx := range s {
		ts.add(&tx)
	}

	return ts
}

func (ts *TransactionsSet) contains(tx *TxPublish) bool {
	_, ok := ts.s[tx]
	return ok
}

func (ts *TransactionsSet) add(tx *TxPublish) {
	ts.s[tx] = true
}

// current set is always modified
func (ts *TransactionsSet) union(other *TransactionsSet) {
	for tx := range other.s {
		ts.s[tx] = true
	}
}

func (ts *TransactionsSet) subtract(other *TransactionsSet) {
	for tx := range other.s {
		if _, ok := ts.s[tx]; ok {
			delete(ts.s, tx)
		}
	}
}
