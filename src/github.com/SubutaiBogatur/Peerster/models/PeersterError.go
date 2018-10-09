package models

type PeersterError struct {
	errorMsg string
}

func (e PeersterError) Error() string {
	return e.errorMsg
}
