package scanner

import (
	"compress/gzip"
	"context"
	"encoding/gob"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/pbnjay/memory"
	"golang.org/x/sys/unix"
	"pkg.jsn.cam/jsn/internal/fsdiff/merkle"
	"pkg.jsn.cam/jsn/internal/fsdiff/snapshot"
	"pkg.jsn.cam/jsn/internal/fsdiff/system"
)

type Config struct {
	IgnorePatterns   []string
	Workers          int
	BufferSize       int
	Verbose          bool
	TraceFiles       bool               // Print every file access
	PreviousSnapshot *snapshot.Snapshot // For incremental mode
	SkipExtendedAttr bool               // Skip xattrs/SELinux/ACLs for faster scans (FSDVR mode)
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

// GetFilesProcessed returns the current count of processed files (atomic read)
func (s *ScanStats) GetFilesProcessed() int64 {
	return atomic.LoadInt64(&s.FilesProcessed)
}

// GetDirsProcessed returns the current count of processed directories (atomic read)
func (s *ScanStats) GetDirsProcessed() int64 {
	return atomic.LoadInt64(&s.DirsProcessed)
}

// ScanResult contains the scan outcome
type ScanResult struct {
	Snapshot    *snapshot.Snapshot
	Interrupted bool
	Error       error
}

// GetStats returns the scanner's current stats for progress tracking
func (s *Scanner) GetStats() *ScanStats {
	return s.stats
}

func New(config *Config) *Scanner {
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
	if config.BufferSize == 0 {
		config.BufferSize = calculateOptimalBufferSize(config.Workers)
	}

	// Increase file descriptor limit
	var rLimit unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_NOFILE, &rLimit); err == nil {
		rLimit.Cur = rLimit.Max
		unix.Setrlimit(unix.RLIMIT_NOFILE, &rLimit)
	}

	walker := newWalker(config.Workers * 2)
	walker.traceFiles = config.TraceFiles
	walker.skipExtendedAttr = config.SkipExtendedAttr

	return &Scanner{
		config:  config,
		stats:   &ScanStats{},
		ignorer: newPathIgnorer(config.IgnorePatterns),
		hasher:  newHasher(config.Workers, config.BufferSize),
		walker:  walker,
	}
}

func (s *Scanner) ScanFilesystem(rootPath string) (*snapshot.Snapshot, error) {
	result := s.ScanFilesystemWithCancel(rootPath)
	return result.Snapshot, result.Error
}

func (s *Scanner) ScanFilesystemWithCancel(rootPath string) *ScanResult {
	s.stats.StartTime = time.Now()

	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	interrupted := false
	go func() {
		sigCount := 0
		for {
			select {
			case <-sigChan:
				sigCount++
				if sigCount == 1 {
					if s.config.Verbose {
						fmt.Println("\n⚠️  Interrupted - saving partial snapshot...")
					}
					interrupted = true
					cancel()
				} else if sigCount >= 2 {
					fmt.Println("\n🛑 Force exit")
					os.Exit(130)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	if s.config.Verbose {
		fmt.Printf("🚀 Starting scan: %d workers, %dKB buffers\n",
			s.config.Workers, s.config.BufferSize/1024)
		if s.config.PreviousSnapshot != nil {
			fmt.Printf("⚡ Incremental mode: using previous snapshot (%d files)\n",
				len(s.config.PreviousSnapshot.Files))
		}
	}

	// Start progress monitor
	progressCtx := make(chan struct{})
	if s.config.Verbose {
		go s.progressMonitor(progressCtx)
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
	err := s.walker.Walk(ctx, rootPath, s.ignorer, s.hasher, results, s.config.PreviousSnapshot)

	close(results)
	collectorWg.Wait()
	close(progressCtx)

	// Build snapshot
	duration := time.Since(s.stats.StartTime)
	snap := &snapshot.Snapshot{
		SystemInfo: system.GetSystemInfo(rootPath),
		Files:      files,
		MerkleRoot: merkle.CalculateMerkleRoot(files),
		IsPartial:  interrupted,
		Stats: snapshot.ScanStats{
			FileCount:    int(atomic.LoadInt64(&s.stats.FilesProcessed)),
			DirCount:     int(atomic.LoadInt64(&s.stats.DirsProcessed)),
			TotalSize:    atomic.LoadInt64(&s.stats.BytesProcessed),
			ErrorCount:   int(atomic.LoadInt64(&s.stats.Errors)),
			ScanDuration: duration,
		},
	}

	if s.config.Verbose {
		if interrupted {
			s.printPartialSummary(snap)
		} else {
			s.printSummary(snap)
		}
	}

	// Don't return ctx.Err() as an error - it's expected on interrupt
	if err != nil && err != context.Canceled {
		return &ScanResult{Snapshot: snap, Interrupted: interrupted, Error: err}
	}

	return &ScanResult{Snapshot: snap, Interrupted: interrupted, Error: nil}
}

// ScanToFile performs a streaming scan that writes directly to a snapshot file
// This keeps memory usage low by never holding all files in memory at once
func (s *Scanner) ScanToFile(rootPath, outputFile string) error {
	result := s.ScanToFileWithCancel(rootPath, outputFile)
	return result.Error
}

func (s *Scanner) ScanToFileWithCancel(rootPath, outputFile string) *ScanResult {
	s.stats.StartTime = time.Now()

	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	interrupted := false
	go func() {
		sigCount := 0
		for {
			select {
			case <-sigChan:
				sigCount++
				if sigCount == 1 {
					if s.config.Verbose {
						fmt.Println("\n⚠️  Interrupted - saving partial snapshot...")
					}
					interrupted = true
					cancel()
				} else if sigCount >= 2 {
					fmt.Println("\n🛑 Force exit")
					os.Exit(130)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	if s.config.Verbose {
		fmt.Printf("🚀 Starting streaming scan: %d workers, %dKB buffers\n",
			s.config.Workers, s.config.BufferSize/1024)
		if s.config.PreviousSnapshot != nil {
			fmt.Printf("⚡ Incremental mode: using previous snapshot (%d files)\n",
				len(s.config.PreviousSnapshot.Files))
		}
	}

	// Create output file
	file, err := os.Create(outputFile)
	if err != nil {
		return &ScanResult{Error: fmt.Errorf("failed to create output file: %v", err)}
	}
	defer file.Close()

	// Create gzip writer for compression
	gzWriter, err := gzip.NewWriterLevel(file, gzip.BestCompression)
	if err != nil {
		return &ScanResult{Error: fmt.Errorf("failed to create gzip writer: %v", err)}
	}
	defer gzWriter.Close()

	// Create snapshot encoder
	encoder := gob.NewEncoder(gzWriter)

	// Start progress monitor
	progressCtx := make(chan struct{})
	if s.config.Verbose {
		go s.progressMonitor(progressCtx)
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
		return &ScanResult{Error: fmt.Errorf("failed to write header: %v", err)}
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

		// Write empty batch as sentinel to mark end of batches
		if err := encoder.Encode([]*snapshot.FileRecord{}); err != nil {
			atomic.AddInt64(&s.stats.Errors, 1)
		}
	}()

	// Walk and process
	err = s.walker.Walk(ctx, rootPath, s.ignorer, s.hasher, results, s.config.PreviousSnapshot)

	close(results)
	collectorWg.Wait()
	close(progressCtx)

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
		return &ScanResult{Error: fmt.Errorf("failed to write final stats: %v", err), Interrupted: interrupted}
	}

	if err := encoder.Encode(rollingMerkleRoot); err != nil {
		return &ScanResult{Error: fmt.Errorf("failed to write merkle root: %v", err), Interrupted: interrupted}
	}

	// Write interrupted flag
	if err := encoder.Encode(interrupted); err != nil {
		return &ScanResult{Error: fmt.Errorf("failed to write interrupted flag: %v", err), Interrupted: interrupted}
	}

	// Ensure all data is written
	if err := gzWriter.Close(); err != nil {
		return &ScanResult{Error: fmt.Errorf("failed to close gzip writer: %v", err), Interrupted: interrupted}
	}

	// Get final snapshot size for reporting
	fileInfo, sizeErr := file.Stat()
	snapshotSize := int64(0)
	if sizeErr == nil {
		snapshotSize = fileInfo.Size()
	}

	if s.config.Verbose {
		if interrupted {
			fmt.Printf("⚠️  Partial scan saved: %d files, %d dirs, %s in %v (%.0f files/sec)\n",
				finalStats.FileCount, finalStats.DirCount,
				formatBytes(finalStats.TotalSize), finalStats.ScanDuration,
				float64(finalStats.FileCount)/finalStats.ScanDuration.Seconds())
		} else {
			fmt.Printf("✅ Streaming scan complete: %d files, %d dirs, %s in %v (%.0f files/sec)\n",
				finalStats.FileCount, finalStats.DirCount,
				formatBytes(finalStats.TotalSize), finalStats.ScanDuration,
				float64(finalStats.FileCount)/finalStats.ScanDuration.Seconds())
		}

		if snapshotSize > 0 {
			compressionRatio := (1.0 - float64(snapshotSize)/float64(finalStats.TotalSize)) * 100
			fmt.Printf("💾 Snapshot saved: %s (%.1f MB, %.3f%% compression)\n",
				outputFile, float64(snapshotSize)/(1024*1024), compressionRatio)
		}

		if finalStats.ErrorCount > 0 {
			fmt.Printf("⚠️  Errors: %d\n", finalStats.ErrorCount)
		}
	}

	// Don't return ctx.Err() - it's expected on interrupt
	if err != nil && err != context.Canceled {
		return &ScanResult{Error: err, Interrupted: interrupted}
	}

	return &ScanResult{Error: nil, Interrupted: interrupted}
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

func (s *Scanner) printPartialSummary(snap *snapshot.Snapshot) {
	fmt.Printf("⚠️  Partial scan: %d files, %d dirs, %s in %v (%.0f files/sec)\n",
		snap.Stats.FileCount, snap.Stats.DirCount,
		formatBytes(snap.Stats.TotalSize), snap.Stats.ScanDuration,
		float64(snap.Stats.FileCount)/snap.Stats.ScanDuration.Seconds())
	fmt.Printf("💡 Use this snapshot with -prev flag to resume faster next time\n")

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

// calculateOptimalBufferSize determines buffer size based on available memory and worker count.
// Returns a value between 256KB (min) and 8MB (max) per worker.
func calculateOptimalBufferSize(workers int) int {
	const (
		minBuffer = 256 * 1024      // 256KB - fewer syscalls
		maxBuffer = 8 * 1024 * 1024 // 8MB - diminishing returns above this
	)

	totalMem := memory.TotalMemory()
	if totalMem == 0 {
		return maxBuffer // fallback to max if we can't detect
	}

	// Use 10% of total memory for buffers
	bufferBudget := totalMem / 10

	// Divide among workers
	perWorker := bufferBudget / uint64(workers)

	// Clamp to bounds
	if perWorker < minBuffer {
		return minBuffer
	}
	if perWorker > maxBuffer {
		return maxBuffer
	}
	return int(perWorker)
}
