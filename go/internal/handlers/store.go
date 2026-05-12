package handlers

import "io"

// Store abstracts the storage backend for put/get/delete operations.
type Store interface {
	Put(key string, body io.Reader) error
	Get(key string) (io.ReadCloser, int64, error)
	Delete(key string) error
	SpaceInfo() (total, used, free int64, err error)
}
