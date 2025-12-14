//go:build linux

package scanner

import (
	"fmt"
	"io"
	"os"

	"github.com/cespare/xxhash/v2"
	"golang.org/x/sys/unix"
)

func (h *Hasher) HashFile(path string, size int64) (string, error) {
	if size == 0 {
		return EmptyHash, nil // Empty file hash
	}

	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Hint sequential access
	unix.Fadvise(int(file.Fd()), 0, 0, unix.FADV_SEQUENTIAL)

	hash := xxhash.New()

	// Strategy based on file size
	switch {
	case size < 65536: // <64KB: Direct read is fastest
		buf := h.bufferPool.Get().([]byte)
		defer h.bufferPool.Put(buf)

		for {
			n, err := file.Read(buf)
			if n > 0 {
				hash.Write(buf[:n])
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", err
			}
		}

	case size > 1048576: // >1MB: Try mmap
		data, err := unix.Mmap(int(file.Fd()), 0, int(size),
			unix.PROT_READ, unix.MAP_PRIVATE|unix.MAP_POPULATE)
		if err == nil {
			defer unix.Munmap(data)
			hash.Write(data)

			// Don't keep large files in cache
			if size > 104857600 { // >100MB
				unix.Fadvise(int(file.Fd()), 0, 0, unix.FADV_DONTNEED)
			}
		} else {
			// Fallback to buffered read
			buf := h.bufferPool.Get().([]byte)
			defer h.bufferPool.Put(buf)
			_, err = io.CopyBuffer(hash, file, buf)
			if err != nil {
				return "", err
			}
		}

	default: // 64KB-1MB: Buffered read
		buf := h.bufferPool.Get().([]byte)
		defer h.bufferPool.Put(buf)
		_, err = io.CopyBuffer(hash, file, buf)
		if err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
