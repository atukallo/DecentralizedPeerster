package config

import "path/filepath"

const (
	LocalIp = "127.0.0.1" // ip addr of gossiper through loopback interface

	MaxPacketSize = 16 * 1024 // in bytes

	FileChunkSize = 8 * 1024 // in bytes
	//FileChunkSize = 64 // in bytes

	SharedFilesPath = "_SharedFiles"
	DownloadsPath   = "_Downloads"

	FileCommonMode = 0755                                 // owner=rwx, all others=rx
	MaxFileSize    = (FileChunkSize / 32) * FileChunkSize // number of hashes * size of chunk
	//MaxFileSize = 100000 // number of hashes * size of chunk
)

var (
	SharedFilesChunksPath = filepath.Join(SharedFilesPath, "chunks")
	DownloadsChunksPath   = filepath.Join(DownloadsPath, "chunks")
)
