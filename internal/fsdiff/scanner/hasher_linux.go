//go:build linux

package scanner

import (
	"fmt"
	"io"
	"os"
	"syscall"

	"github.com/cespare/xxhash/v2"
	"golang.org/x/sys/unix"
)

func (h *Hasher) HashFile(path string, size int64) (string, error) {
	if size == 0 {
		return EmptyHash, nil // Empty file hash
	}

	// Use O_NOATIME to avoid inode write on every read
	// Falls back to regular open if we don't have permission (non-owner)
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOATIME, 0)
	if err != nil {
		// Fallback without O_NOATIME
		fd, err = unix.Open(path, unix.O_RDONLY, 0)
		if err != nil {
			return "", err
		}
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()

	// Hint sequential access
	unix.Fadvise(fd, 0, 0, unix.FADV_SEQUENTIAL)

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

	case size > 4194304: // >4MB: Use mmap
		// MAP_PRIVATE only - removed MAP_POPULATE to avoid blocking on page faults
		// Let kernel stream pages on demand
		data, err := unix.Mmap(fd, 0, int(size),
			unix.PROT_READ, unix.MAP_PRIVATE)
		if err == nil {
			// MADV_SEQUENTIAL hints kernel to read ahead and drop behind
			unix.Madvise(data, syscall.MADV_SEQUENTIAL)

			hash.Write(data)
			unix.Munmap(data)

			// Don't keep large files in cache
			if size > 104857600 { // >100MB
				unix.Fadvise(fd, 0, 0, unix.FADV_DONTNEED)
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

	default: // 64KB-4MB: Buffered read
		buf := h.bufferPool.Get().([]byte)
		defer h.bufferPool.Put(buf)
		_, err = io.CopyBuffer(hash, file, buf)
		if err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
