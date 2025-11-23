package scanner

import (
	"sync"
)

const EmptyHash = "ef46db3751d8e999" // generated using xxh64sum with nothing as an input
type Hasher struct {
	bufferPool *sync.Pool
	workers    int
}

func newHasher(workers, bufferSize int) *Hasher {
	return &Hasher{
		workers: workers,
		bufferPool: &sync.Pool{
			New: func() interface{} {
				return make([]byte, bufferSize)
			},
		},
	}
}
