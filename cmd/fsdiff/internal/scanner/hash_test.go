package scanner

import (
	"crypto/sha256"
	"github.com/cespare/xxhash/v2"
	"testing"
)

func BenchmarkOldHash(b *testing.B) {
	data := []byte("test data")
	for i := 0; i < b.N; i++ {
		sha256.Sum256(data)
	}
}

func BenchmarkNewHash(b *testing.B) {
	data := []byte("test data")
	for i := 0; i < b.N; i++ {
		xxhash.Sum64(data)
	}
}
