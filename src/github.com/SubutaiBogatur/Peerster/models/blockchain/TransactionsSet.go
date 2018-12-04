package blockchain

import . "github.com/SubutaiBogatur/Peerster/models"

// it's owful, that go doesn't have generics/templates
type TransactionsSet struct {
	s map[string]*TxPublish // filename -> tx
}

func InitEmpty() *TransactionsSet {
	return &TransactionsSet{s: make(map[string]*TxPublish)}
}

func InitFromSlice(s []TxPublish) *TransactionsSet {
	ts := InitEmpty()
	for _, tx := range s {
		ts.add(&tx)
	}

	return ts
}

func (ts *TransactionsSet) containsFilename(filename string) bool {
	_, ok := ts.s[filename]
	return ok
}

func (ts *TransactionsSet) contains(tx *TxPublish) bool {
	return ts.containsFilename(tx.File.Name)
}

func (ts *TransactionsSet) get(filename string) *TxPublish {
	return ts.s[filename]
}

func (ts *TransactionsSet) add(tx *TxPublish) bool {
	_, ok := ts.s[tx.File.Name]
	if ok {
		return false
	}

	ts.s[tx.File.Name] = tx
	return true
}

// current set is always modified
func (ts *TransactionsSet) union(other *TransactionsSet) {
	for name, tx := range other.s {
		ts.s[name] = tx
	}
}

func (ts *TransactionsSet) subtract(other *TransactionsSet) {
	for name := range other.s {
		if _, ok := ts.s[name]; ok {
			delete(ts.s, name)
		}
	}
}

func (ts *TransactionsSet) getSliceCopy() []TxPublish {
	ret := make([]TxPublish, len(ts.s))
	for _, tx := range ts.s {
		ret = append(ret, *tx)
	}
	return ret
}

func (ts *TransactionsSet) isEmpty() bool {
	return len(ts.s) == 0
}

func (ts *TransactionsSet) clear() {
	ts.s = make(map[string]*TxPublish)
}
