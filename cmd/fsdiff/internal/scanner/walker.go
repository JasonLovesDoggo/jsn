package scanner

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"pkg.jsn.cam/jsn/cmd/fsdiff/internal/snapshot"
	systemv2 "pkg.jsn.cam/jsn/cmd/fsdiff/internal/system/v2"
)

type Walker struct {
	dirQueue chan string
	fileJobs chan FileJob
	results  chan<- *FileResult
	workers  int
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
		dirQueue: make(chan string, 1000),
		fileJobs: make(chan FileJob, queueSize),
		workers:  0,
	}
}

func (w *Walker) Walk(root string, ignorer *PathIgnorer, hasher *Hasher, results chan<- *FileResult, prevSnapshot *snapshot.Snapshot) error {
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

	// Use atomic counter for active directories
	var activeDirs int64 = 1
	var dirMutex sync.Mutex
	dirClosed := false

	// Start directory workers
	var dirWg sync.WaitGroup
	numDirWorkers := 4

	for i := 0; i < numDirWorkers; i++ {
		dirWg.Add(1)
		go w.dirWorker(&dirWg, ignorer, &activeDirs, &dirMutex, &dirClosed, prevSnapshot)
	}

	// Start file workers
	var fileWg sync.WaitGroup
	for i := 0; i < hasher.workers; i++ {
		fileWg.Add(1)
		go w.fileWorker(&fileWg, hasher, results)
	}

	// Seed with root
	w.dirQueue <- root

	// Wait for all directories to be processed
	dirWg.Wait()
	close(w.fileJobs)

	// Wait for all files to be processed
	fileWg.Wait()

	return nil
}

func (w *Walker) dirWorker(wg *sync.WaitGroup, ignorer *PathIgnorer, activeDirs *int64, dirMutex *sync.Mutex, dirClosed *bool, prevSnapshot *snapshot.Snapshot) {
	defer wg.Done()

	for path := range w.dirQueue {
		entries, err := os.ReadDir(path)
		if err != nil {
			w.decrementAndMaybeClose(activeDirs, dirMutex, dirClosed)
			continue
		}

		// Collect subdirs first so we can batch increment activeDirs
		var subdirs []string
		for _, entry := range entries {
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
				w.results <- &FileResult{Record: &snapshot.FileRecord{
					Path:     fullPath,
					Size:     0,
					Mode:     info.Mode(),
					ModTime:  info.ModTime(),
					IsDir:    true,
					FileInfo: systemv2.GetFileInfo(fullPath, info),
				}}
				subdirs = append(subdirs, fullPath)
			} else {
				// Look up previous record for incremental mode
				var prev *snapshot.FileRecord
				if prevSnapshot != nil {
					prev, _ = prevSnapshot.Files[fullPath]
				}
				w.fileJobs <- FileJob{Path: fullPath, Info: info, Prev: prev}
			}
		}

		// Batch increment for all subdirs, then queue them
		if len(subdirs) > 0 {
			atomic.AddInt64(activeDirs, int64(len(subdirs)))
			for _, subdir := range subdirs {
				w.dirQueue <- subdir
			}
		}

		w.decrementAndMaybeClose(activeDirs, dirMutex, dirClosed)
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

func (w *Walker) fileWorker(wg *sync.WaitGroup, hasher *Hasher, results chan<- *FileResult) {
	defer wg.Done()

	for job := range w.fileJobs {
		record := &snapshot.FileRecord{
			Path:     job.Path,
			Size:     job.Info.Size(),
			Mode:     job.Info.Mode(),
			ModTime:  job.Info.ModTime(),
			IsDir:    false,
			FileInfo: systemv2.GetFileInfo(job.Path, job.Info),
		}

		// Hash regular files
		if job.Info.Mode().IsRegular() {
			// Incremental optimization: skip hashing if mtime+size unchanged
			if job.Prev != nil &&
				job.Prev.ModTime.Equal(job.Info.ModTime()) &&
				job.Prev.Size == job.Info.Size() {
				record.Hash = job.Prev.Hash
			} else {
				hash, err := hasher.HashFile(job.Path, job.Info.Size())
				if err != nil {
					record.Hash = "ERROR"
				} else {
					record.Hash = hash
				}
			}
		}

		results <- &FileResult{Record: record}
	}
}
