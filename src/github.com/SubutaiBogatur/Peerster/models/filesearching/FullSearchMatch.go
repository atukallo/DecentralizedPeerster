package filesearching

import "sync"

type FullSearchMatch struct {
	Filename     string
	Origin       string
	MetafileHash [32]byte

	mux sync.Mutex
}
