//go:build !linux

package scanner

import (
	"fmt"
	"io"
	"os"

	"github.com/cespare/xxhash/v2"
)

func (h *Hasher) HashFile(path string, size int64) (string, error) {
	if size == 0 {
		return EmptyHash, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Grab buffer from pool
	buf := h.bufferPool.Get().([]byte)
	defer h.bufferPool.Put(buf)

	hh := xxhash.New()

	for {
		n, err := f.Read(buf)
		if n > 0 {
			_, _ = hh.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("%x", hh.Sum(nil)), nil
}
