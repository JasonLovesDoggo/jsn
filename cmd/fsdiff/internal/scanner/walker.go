package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"pkg.jsn.cam/jsn/cmd/fsdiff/internal/snapshot"
	systemv2 "pkg.jsn.cam/jsn/cmd/fsdiff/internal/system/v2"
)

type Walker struct {
	dirQueue   chan string
	fileJobs   chan FileJob
	results    chan<- *FileResult
	workers    int
	traceFiles bool
}

type FileJob struct {
	Info os.FileInfo
	Path string
	Prev *snapshot.FileRecord // Previous record for incremental mode
}

type FileResult struct {
	Record *snapshot.FileRecord
	Error  error
}

func newWalker(queueSize int) *Walker {
	return &Walker{
		dirQueue: make(chan string, 100000), // Large buffer to keep workers busy; sync fallback prevents deadlock
		fileJobs: make(chan FileJob, queueSize),
		workers:  0,
	}
}

func (w *Walker) Walk(ctx context.Context, root string, ignorer *PathIgnorer, hasher *Hasher, results chan<- *FileResult, prevSnapshot *snapshot.Snapshot) error {
	w.results = results

	rootInfo, err := os.Stat(root)
	if err != nil {
		return err
	}

	results <- &FileResult{Record: &snapshot.FileRecord{
		Path:     root,
		Size:     0,
		Mode:     rootInfo.Mode(),
		ModTime:  rootInfo.ModTime(),
		IsDir:    true,
		FileInfo: systemv2.GetFileInfo(root, rootInfo),
	}}

	var activeDirs int64 = 1
	var dirMutex sync.Mutex
	dirClosed := false

	var dirWg sync.WaitGroup
	numDirWorkers := 4

	for i := 0; i < numDirWorkers; i++ {
		dirWg.Add(1)
		go w.dirWorker(ctx, &dirWg, ignorer, &activeDirs, &dirMutex, &dirClosed, prevSnapshot)
	}

	var fileWg sync.WaitGroup
	for i := 0; i < hasher.workers; i++ {
		fileWg.Add(1)
		go w.fileWorker(ctx, &fileWg, hasher, results)
	}

	// Debug monitor for hangs
	if w.traceFiles {
		go func() {
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					active := atomic.LoadInt64(&activeDirs)
					fmt.Printf("🔍 DEBUG: activeDirs=%d dirQueue=%d fileJobs=%d\n",
						active, len(w.dirQueue), len(w.fileJobs))
					if active == 0 {
						return
					}
				}
			}
		}()
	}

	w.dirQueue <- root

	dirWg.Wait()
	close(w.fileJobs)
	fileWg.Wait()

	return ctx.Err()
}

func (w *Walker) dirWorker(ctx context.Context, wg *sync.WaitGroup, ignorer *PathIgnorer, activeDirs *int64, dirMutex *sync.Mutex, dirClosed *bool, prevSnapshot *snapshot.Snapshot) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			// Cancelled - help close the queue if we're the last one
			w.decrementAndMaybeClose(activeDirs, dirMutex, dirClosed)
			return
		case path, ok := <-w.dirQueue:
			if !ok {
				return // Channel closed
			}

			// Check cancellation before processing
			if ctx.Err() != nil {
				w.decrementAndMaybeClose(activeDirs, dirMutex, dirClosed)
				return
			}

			if w.traceFiles {
				fmt.Printf("→ ENTER %s\n", path)
			}

			entries, err := os.ReadDir(path)
			if err != nil {
				if w.traceFiles {
					fmt.Printf("✗ ERROR %s: %v\n", path, err)
				}
				w.decrementAndMaybeClose(activeDirs, dirMutex, dirClosed)
				continue
			}

			if w.traceFiles {
				fmt.Printf("✓ READ  %s (%d entries)\n", path, len(entries))
			}

			// Collect subdirs to process after files
			var subdirs []string
			for _, entry := range entries {
				// Check cancellation periodically in large directories
				if ctx.Err() != nil {
					break
				}

				fullPath := filepath.Join(path, entry.Name())
				if ignorer.ShouldIgnore(fullPath) {
					continue
				}

				info, err := entry.Info()
				if err != nil {
					continue
				}

				if entry.IsDir() {
					// Send directory record (blocking - never drop)
					select {
					case <-ctx.Done():
						// Cancelled while waiting to send
					case w.results <- &FileResult{Record: &snapshot.FileRecord{
						Path:     fullPath,
						Size:     0,
						Mode:     info.Mode(),
						ModTime:  info.ModTime(),
						IsDir:    true,
						FileInfo: systemv2.GetFileInfo(fullPath, info),
					}}:
					}
					subdirs = append(subdirs, fullPath)
				} else {
					// Look up previous record for incremental mode
					var prev *snapshot.FileRecord
					if prevSnapshot != nil {
						prev, _ = prevSnapshot.Files[fullPath]
					}
					select {
					case <-ctx.Done():
						// Cancelled while waiting to send
					case w.fileJobs <- FileJob{Path: fullPath, Info: info, Prev: prev}:
					}
				}
			}

			// Queue subdirs - increment counter only on successful queue
			// If queue is full, process synchronously to avoid deadlock
			for _, subdir := range subdirs {
				if ctx.Err() != nil {
					break
				}
				// Try non-blocking send first
				select {
				case w.dirQueue <- subdir:
					// Successfully queued - now safe to increment
					atomic.AddInt64(activeDirs, 1)
				default:
					// Queue full - process synchronously to avoid deadlock
					// No increment needed since we process it ourselves
					w.processDir(ctx, subdir, ignorer, activeDirs, dirMutex, dirClosed, prevSnapshot)
				}
			}

			w.decrementAndMaybeClose(activeDirs, dirMutex, dirClosed)
		}
	}
}

// processDir handles a directory synchronously when the queue is full.
// This prevents deadlock by not requiring the queue for progress.
// Always tries to re-queue subdirs to return to parallel mode ASAP.
func (w *Walker) processDir(ctx context.Context, path string, ignorer *PathIgnorer, activeDirs *int64, dirMutex *sync.Mutex, dirClosed *bool, prevSnapshot *snapshot.Snapshot) {
	if ctx.Err() != nil {
		return
	}

	if w.traceFiles {
		fmt.Printf("→ SYNC  %s\n", path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		if w.traceFiles {
			fmt.Printf("✗ ERROR %s: %v\n", path, err)
		}
		return
	}

	// Collect subdirs, try to push as many as possible back to queue
	var subdirs []string
	for _, entry := range entries {
		if ctx.Err() != nil {
			return
		}

		fullPath := filepath.Join(path, entry.Name())
		if ignorer.ShouldIgnore(fullPath) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if entry.IsDir() {
			// Send directory record
			select {
			case <-ctx.Done():
				return
			case w.results <- &FileResult{Record: &snapshot.FileRecord{
				Path:     fullPath,
				Size:     0,
				Mode:     info.Mode(),
				ModTime:  info.ModTime(),
				IsDir:    true,
				FileInfo: systemv2.GetFileInfo(fullPath, info),
			}}:
			}
			subdirs = append(subdirs, fullPath)
		} else {
			var prev *snapshot.FileRecord
			if prevSnapshot != nil {
				prev, _ = prevSnapshot.Files[fullPath]
			}
			select {
			case <-ctx.Done():
				return
			case w.fileJobs <- FileJob{Path: fullPath, Info: info, Prev: prev}:
			}
		}
	}

	// Try to queue ALL subdirs back for parallel processing
	// Only recurse synchronously if queue is still full
	for _, subdir := range subdirs {
		if ctx.Err() != nil {
			return
		}
		select {
		case w.dirQueue <- subdir:
			// Back to parallel mode
			atomic.AddInt64(activeDirs, 1)
		default:
			// Still full, process this one synchronously
			w.processDir(ctx, subdir, ignorer, activeDirs, dirMutex, dirClosed, prevSnapshot)
		}
	}
}

func (w *Walker) decrementAndMaybeClose(activeDirs *int64, dirMutex *sync.Mutex, dirClosed *bool) {
	if atomic.AddInt64(activeDirs, -1) == 0 {
		dirMutex.Lock()
		if !*dirClosed {
			*dirClosed = true
			close(w.dirQueue)
		}
		dirMutex.Unlock()
	}
}

func (w *Walker) fileWorker(ctx context.Context, wg *sync.WaitGroup, hasher *Hasher, results chan<- *FileResult) {
	defer wg.Done()

	for job := range w.fileJobs {
		// Check cancellation - but finish if we already started
		if ctx.Err() != nil {
			// Still process this job to avoid data loss, then exit
		}

		if w.traceFiles {
			fmt.Printf("  → FILE %s\n", job.Path)
		}

		var fileInfo *systemv2.FileInfo
		if w.traceFiles {
			fmt.Printf("    STAT %s\n", job.Path)
		}
		fileInfo = systemv2.GetFileInfo(job.Path, job.Info)
		if w.traceFiles {
			fmt.Printf("    DONE %s\n", job.Path)
		}

		record := &snapshot.FileRecord{
			Path:     job.Path,
			Size:     job.Info.Size(),
			Mode:     job.Info.Mode(),
			ModTime:  job.Info.ModTime(),
			IsDir:    false,
			FileInfo: fileInfo,
		}

		// Hash regular files
		if job.Info.Mode().IsRegular() {
			// Incremental optimization: skip hashing if mtime+size unchanged
			if job.Prev != nil &&
				job.Prev.ModTime.Equal(job.Info.ModTime()) &&
				job.Prev.Size == job.Info.Size() {
				record.Hash = job.Prev.Hash
				if w.traceFiles {
					fmt.Printf("  ⚡ SKIP %s (unchanged)\n", job.Path)
				}
			} else {
				if w.traceFiles {
					fmt.Printf("  # HASH %s (%d bytes)\n", job.Path, job.Info.Size())
				}
				hash, err := hasher.HashFile(job.Path, job.Info.Size())
				if err != nil {
					record.Hash = "ERROR"
					if w.traceFiles {
						fmt.Printf("  ✗ FAIL %s: %v\n", job.Path, err)
					}
				} else {
					record.Hash = hash
				}
			}
		}

		select {
		case <-ctx.Done():
			// Still send the result we computed
			select {
			case results <- &FileResult{Record: record}:
			default:
				// Channel full and cancelled, drop it
			}
			return
		case results <- &FileResult{Record: record}:
		}
	}
}
