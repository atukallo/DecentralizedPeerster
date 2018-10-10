package models

type PeersterError struct {
	ErrorMsg string
}

func (e PeersterError) Error() string {
	return e.ErrorMsg
}
