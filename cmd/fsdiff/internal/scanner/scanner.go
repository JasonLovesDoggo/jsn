package scanner

import (
	"compress/gzip"
	"encoding/gob"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"time"

	"pkg.jsn.cam/jsn/cmd/fsdiff/internal/merkle"

	"golang.org/x/sys/unix"
	"pkg.jsn.cam/jsn/cmd/fsdiff/internal/snapshot"
	"pkg.jsn.cam/jsn/cmd/fsdiff/internal/system"
)

type Config struct {
	IgnorePatterns []string
	Workers        int
	BufferSize     int
	Verbose        bool
}

type Scanner struct {
	config  *Config
	stats   *ScanStats
	ignorer *PathIgnorer
	hasher  *Hasher
	walker  *Walker
}

type ScanStats struct {
	StartTime      time.Time
	FilesProcessed int64
	DirsProcessed  int64
	BytesProcessed int64
	Errors         int64
}

func New(config *Config) *Scanner {
	if config.BufferSize == 0 {
		config.BufferSize = 256 * 1024
	}
	if config.Workers == 0 {
		// Optimize for memory efficiency while maintaining speed
		// More workers = faster but exponentially more memory
		cores := runtime.NumCPU()
		if cores <= 4 {
			config.Workers = cores * 2 // 8 workers max on small systems
		} else if cores <= 8 {
			config.Workers = cores + 4 // 12 workers max on medium systems
		} else {
			config.Workers = cores // Cap at core count for large systems
		}
	}

	// Increase file descriptor limit
	var rLimit unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_NOFILE, &rLimit); err == nil {
		rLimit.Cur = rLimit.Max
		unix.Setrlimit(unix.RLIMIT_NOFILE, &rLimit)
	}

	return &Scanner{
		config:  config,
		stats:   &ScanStats{},
		ignorer: newPathIgnorer(config.IgnorePatterns),
		hasher:  newHasher(config.Workers, config.BufferSize),
		walker:  newWalker(config.Workers * 2),
	}
}

func (s *Scanner) ScanFilesystem(rootPath string) (*snapshot.Snapshot, error) {
	s.stats.StartTime = time.Now()

	if s.config.Verbose {
		fmt.Printf("🚀 Starting scan: %d workers, %dKB buffers\n",
			s.config.Workers, s.config.BufferSize/1024)
	}

	// Start progress monitor
	ctx := make(chan struct{})
	if s.config.Verbose {
		go s.progressMonitor(ctx)
	}

	// Start result collector
	results := make(chan *FileResult, s.config.Workers*2)
	files := make(map[string]*snapshot.FileRecord)

	var collectorWg sync.WaitGroup
	collectorWg.Add(1)
	go func() {
		defer collectorWg.Done()
		for result := range results {
			if result.Error != nil {
				atomic.AddInt64(&s.stats.Errors, 1)
				continue
			}
			files[result.Record.Path] = result.Record

			if result.Record.IsDir {
				atomic.AddInt64(&s.stats.DirsProcessed, 1)
			} else {
				atomic.AddInt64(&s.stats.FilesProcessed, 1)
				atomic.AddInt64(&s.stats.BytesProcessed, result.Record.Size)
			}
		}
	}()

	// Walk and process
	err := s.walker.Walk(rootPath, s.ignorer, s.hasher, results)

	close(results)
	collectorWg.Wait()
	close(ctx)

	// Build snapshot
	duration := time.Since(s.stats.StartTime)
	snap := &snapshot.Snapshot{
		SystemInfo: system.GetSystemInfo(rootPath),
		Files:      files,
		MerkleRoot: merkle.CalculateMerkleRoot(files),
		Stats: snapshot.ScanStats{
			FileCount:    int(atomic.LoadInt64(&s.stats.FilesProcessed)),
			DirCount:     int(atomic.LoadInt64(&s.stats.DirsProcessed)),
			TotalSize:    atomic.LoadInt64(&s.stats.BytesProcessed),
			ErrorCount:   int(atomic.LoadInt64(&s.stats.Errors)),
			ScanDuration: duration,
		},
	}

	if s.config.Verbose {
		s.printSummary(snap)
	}

	return snap, err
}

// ScanToFile performs a streaming scan that writes directly to a snapshot file
// This keeps memory usage low by never holding all files in memory at once
func (s *Scanner) ScanToFile(rootPath, outputFile string) error {
	s.stats.StartTime = time.Now()

	if s.config.Verbose {
		fmt.Printf("🚀 Starting streaming scan: %d workers, %dKB buffers\n",
			s.config.Workers, s.config.BufferSize/1024)
	}

	// Create output file
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer file.Close()

	// Create gzip writer for compression
	gzWriter, err := gzip.NewWriterLevel(file, gzip.BestCompression)
	if err != nil {
		return fmt.Errorf("failed to create gzip writer: %v", err)
	}
	defer gzWriter.Close()

	// Create snapshot encoder
	encoder := gob.NewEncoder(gzWriter)

	// Start progress monitor
	ctx := make(chan struct{})
	if s.config.Verbose {
		go s.progressMonitor(ctx)
	}

	// Create header with system info
	systemInfo := system.GetSystemInfo(rootPath)
	header := &snapshot.Snapshot{
		Version:    "streaming", // Special version for streaming snapshots
		SystemInfo: systemInfo,
		Files:      nil,                  // Will be populated incrementally
		Stats:      snapshot.ScanStats{}, // Will be updated at the end
	}

	// Write header (we'll update stats later)
	if err := encoder.Encode(header); err != nil {
		return fmt.Errorf("failed to write header: %v", err)
	}

	// Start result collector with memory-limited batch and rolling merkle calculation
	results := make(chan *FileResult, s.config.Workers*2)
	const batchSize = 10000 // Process files in batches of 10k
	batch := make([]*snapshot.FileRecord, 0, batchSize)
	// Use rolling XOR for merkle root calculation to avoid accumulating all hashes
	var rollingMerkleRoot uint64 = 0
	batchCount := 0

	var collectorWg sync.WaitGroup
	collectorWg.Add(1)
	go func() {
		defer collectorWg.Done()

		for result := range results {
			if result.Error != nil {
				atomic.AddInt64(&s.stats.Errors, 1)
				continue
			}

			// Add to current batch
			batch = append(batch, result.Record)
			// Rolling XOR for merkle calculation - no memory accumulation
			rollingMerkleRoot ^= merkle.HashRecord(result.Record)

			// Update stats
			if result.Record.IsDir {
				atomic.AddInt64(&s.stats.DirsProcessed, 1)
			} else {
				atomic.AddInt64(&s.stats.FilesProcessed, 1)
				atomic.AddInt64(&s.stats.BytesProcessed, result.Record.Size)
			}

			// Write batch when full
			if len(batch) >= batchSize {
				if err := encoder.Encode(batch); err != nil {
					atomic.AddInt64(&s.stats.Errors, 1)
				}
				batch = batch[:0] // Reset batch, reuse underlying array
				batchCount++

				// Force flush to disk every 10 batches to prevent gzip buffer buildup
				if batchCount%10 == 0 {
					gzWriter.Flush()
				}
				runtime.GC() // Force GC after each batch
			}
		}

		// Write final batch
		if len(batch) > 0 {
			if err := encoder.Encode(batch); err != nil {
				atomic.AddInt64(&s.stats.Errors, 1)
			}
		}
	}()

	// Walk and process
	err = s.walker.Walk(rootPath, s.ignorer, s.hasher, results)

	close(results)
	collectorWg.Wait()
	close(ctx)

	// Write final stats
	duration := time.Since(s.stats.StartTime)
	finalStats := snapshot.ScanStats{
		FileCount:    int(atomic.LoadInt64(&s.stats.FilesProcessed)),
		DirCount:     int(atomic.LoadInt64(&s.stats.DirsProcessed)),
		TotalSize:    atomic.LoadInt64(&s.stats.BytesProcessed),
		ErrorCount:   int(atomic.LoadInt64(&s.stats.Errors)),
		ScanDuration: duration,
	}

	if err := encoder.Encode(finalStats); err != nil {
		return fmt.Errorf("failed to write final stats: %v", err)
	}

	if err := encoder.Encode(rollingMerkleRoot); err != nil {
		return fmt.Errorf("failed to write merkle root: %v", err)
	}

	// Ensure all data is written
	if err := gzWriter.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %v", err)
	}

	// Get final snapshot size for reporting
	fileInfo, sizeErr := file.Stat()
	snapshotSize := int64(0)
	if sizeErr == nil {
		snapshotSize = fileInfo.Size()
	}

	if s.config.Verbose {
		fmt.Printf("✅ Streaming scan complete: %d files, %d dirs, %s in %v (%.0f files/sec)\n",
			finalStats.FileCount, finalStats.DirCount,
			formatBytes(finalStats.TotalSize), finalStats.ScanDuration,
			float64(finalStats.FileCount)/finalStats.ScanDuration.Seconds())

		if snapshotSize > 0 {
			compressionRatio := (1.0 - float64(snapshotSize)/float64(finalStats.TotalSize)) * 100
			fmt.Printf("💾 Snapshot saved: %s (%.1f MB, %.3f%% compression)\n",
				outputFile, float64(snapshotSize)/(1024*1024), compressionRatio)
		}

		if finalStats.ErrorCount > 0 {
			fmt.Printf("⚠️  Errors: %d\n", finalStats.ErrorCount)
		}
	}

	return err
}

func (s *Scanner) progressMonitor(ctx <-chan struct{}) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Memory profiling setup
	memCount := 0
	var lastMemUsage uint64

	for {
		select {
		case <-ctx:
			return
		case <-ticker.C:
			files := atomic.LoadInt64(&s.stats.FilesProcessed)
			dirs := atomic.LoadInt64(&s.stats.DirsProcessed)
			bytes := atomic.LoadInt64(&s.stats.BytesProcessed)
			elapsed := time.Since(s.stats.StartTime)
			rate := float64(files+dirs) / elapsed.Seconds()

			// Get current memory stats (Go heap + system memory)
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			goHeapMB := m.Alloc / 1024 / 1024 // Go heap in MB
			totalMemMB := m.Sys / 1024 / 1024 // Total system memory in MB (includes mmap, stacks, etc.)

			// Check for significant memory increase (use total system memory)
			memDiff := int64(totalMemMB) - int64(lastMemUsage)
			if memDiff > 100 { // Alert if memory increases by more than 100MB
				fmt.Printf("🚨 MEMORY SPIKE: +%d MB (heap: %d MB, total: %d MB)\n", memDiff, goHeapMB, totalMemMB)

				// Create memory profile on significant spike
				memCount++
				filename := fmt.Sprintf("fsdiff-mem-profile-%d.prof", memCount)
				if f, err := os.Create(filename); err == nil {
					pprof.WriteHeapProfile(f)
					f.Close()
					fmt.Printf("💾 Memory profile saved: %s\n", filename)
				}
			}

			fmt.Printf("📊 %d files, %d dirs, %s | %.0f items/sec | %d MB heap / %d MB total\n",
				files, dirs, formatBytes(bytes), rate, goHeapMB, totalMemMB)

			lastMemUsage = totalMemMB
		}
	}
}

func (s *Scanner) printSummary(snap *snapshot.Snapshot) {
	fmt.Printf("✅ Scan complete: %d files, %d dirs, %s in %v (%.0f files/sec)\n",
		snap.Stats.FileCount, snap.Stats.DirCount,
		formatBytes(snap.Stats.TotalSize), snap.Stats.ScanDuration,
		float64(snap.Stats.FileCount)/snap.Stats.ScanDuration.Seconds())

	if snap.Stats.ErrorCount > 0 {
		fmt.Printf("⚠️  Errors: %d\n", snap.Stats.ErrorCount)
	}
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
