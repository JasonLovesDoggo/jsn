package scanner

import (
	"fmt"
	"github.com/cespare/xxhash/v2"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"pkg.jsn.cam/jsn/cmd/fsdiff/internal/snapshot"
	"pkg.jsn.cam/jsn/cmd/fsdiff/internal/system"
)

// Config holds scanner configuration
type Config struct {
	Workers        int
	Verbose        bool
	IgnorePatterns []string
	BufferSize     int
}

// Scanner handles parallel filesystem scanning
type Scanner struct {
	config  *Config
	stats   *ScanStats
	ignorer *PathIgnorer
}

// ScanStats tracks scanning progress and statistics
type ScanStats struct {
	FilesProcessed int64
	DirsProcessed  int64
	BytesProcessed int64
	Errors         int64
	StartTime      time.Time
	LastUpdate     time.Time
	mutex          sync.RWMutex
}

// PathIgnorer handles ignore pattern matching
type PathIgnorer struct {
	patterns []string
	defaults []string
}

// FileJob represents a file to be processed
type FileJob struct {
	Path string
	Info fs.DirEntry
}

// FileResult represents the result of processing a file
type FileResult struct {
	Record *snapshot.FileRecord
	Error  error
}

// New creates a new scanner with the given configuration
func New(config *Config) *Scanner {
	if config.BufferSize == 0 {
		config.BufferSize = 64 * 1024 // 64KB default buffer
	}

	defaultIgnores := []string{
		"/proc", "/sys", "/dev", "/tmp", "/var/tmp", "/run", "/var/run",
		"/var/log", "/var/cache", "/var/lib/dhcp", "/var/lib/systemd",
		"/boot/grub", "/.cache", "node_modules", "*.log", "*.tmp",
		"/home/*/.cache", "/home/*/.local/share/Trash",
		"/home/*/.mozilla/firefox/*/Cache",
		"/home/*/.config/google-chrome/*/Cache",
		"/var/lib/docker/overlay2", "/var/lib/containerd",
		".git", ".svn", ".hg", "__pycache__", ".pytest_cache",
		"*.pyc", "*.pyo", "*.swp", "*.bak", "*~",
	}

	ignorer := &PathIgnorer{
		patterns: config.IgnorePatterns,
		defaults: defaultIgnores,
	}

	return &Scanner{
		config:  config,
		stats:   &ScanStats{},
		ignorer: ignorer,
	}
}

// ScanFilesystem performs a parallel scan of the filesystem
func (s *Scanner) ScanFilesystem(rootPath string) (*snapshot.Snapshot, error) {
	s.stats.StartTime = time.Now()
	s.stats.LastUpdate = s.stats.StartTime

	fmt.Printf("🚀 Starting parallel scan with %d workers...\n", s.config.Workers)

	// Create channels for work distribution
	jobs := make(chan FileJob, s.config.Workers*10)
	results := make(chan FileResult, s.config.Workers*10)

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < s.config.Workers; i++ {
		wg.Add(1)
		go s.worker(i, jobs, results, &wg)
	}

	// Start progress monitor
	ctx := make(chan struct{})
	go s.progressMonitor(ctx)

	// Start result collector
	files := make(map[string]*snapshot.FileRecord)
	var collectorWg sync.WaitGroup
	collectorWg.Add(1)
	go s.resultCollector(results, files, &collectorWg)

	// Walk filesystem and distribute work
	walkStart := time.Now()
	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if s.config.Verbose {
				fmt.Printf("⚠️  Walk error for %s: %v\n", path, err)
			}
			atomic.AddInt64(&s.stats.Errors, 1)
			return nil // Continue walking
		}

		// Check if path should be ignored
		if s.ignorer.ShouldIgnore(path) {
			if d.IsDir() {
				if s.config.Verbose {
					fmt.Printf("🚫 Skipping directory: %s\n", path)
				}
				return filepath.SkipDir
			}
			return nil
		}

		// Send job to workers
		select {
		case jobs <- FileJob{Path: path, Info: d}:
		case <-time.After(time.Second):
			// Prevent deadlock if workers are slow
			if s.config.Verbose {
				fmt.Printf("⚠️  Worker queue full, skipping: %s\n", path)
			}
			atomic.AddInt64(&s.stats.Errors, 1)
		}

		return nil
	})

	walkDuration := time.Since(walkStart)
	if s.config.Verbose {
		fmt.Printf("📁 Filesystem walk completed in %v\n", walkDuration)
	}

	// Close jobs channel and wait for workers
	close(jobs)
	wg.Wait()

	// Close results and wait for collector
	close(results)
	collectorWg.Wait()

	// Stop progress monitor
	close(ctx)

	// Create a simple merkle root hash (avoiding complex tree building)
	fmt.Printf("🌳 Calculating merkle root...\n")
	merkleRoot := s.calculateSimpleMerkleRoot(files)

	// Gather system information
	sysInfo := system.GetSystemInfo(rootPath)
	totalDuration := time.Since(s.stats.StartTime)
	sysInfo.ScanDuration = totalDuration

	// Create snapshot
	snap := &snapshot.Snapshot{
		SystemInfo: sysInfo,
		Files:      files,
		MerkleRoot: merkleRoot,
		Tree:       nil, // We'll skip the complex tree for now
		Stats: snapshot.ScanStats{
			FileCount:    int(atomic.LoadInt64(&s.stats.FilesProcessed)),
			DirCount:     int(atomic.LoadInt64(&s.stats.DirsProcessed)),
			TotalSize:    atomic.LoadInt64(&s.stats.BytesProcessed),
			ErrorCount:   int(atomic.LoadInt64(&s.stats.Errors)),
			ScanDuration: totalDuration,
		},
	}

	fmt.Printf("✅ Scan completed successfully!\n")
	fmt.Printf("   📊 Stats: %d files, %d dirs, %s processed\n",
		snap.Stats.FileCount, snap.Stats.DirCount, formatBytes(snap.Stats.TotalSize))
	fmt.Printf("   ⏱️  Duration: %v (%.0f files/sec)\n",
		totalDuration, float64(snap.Stats.FileCount)/totalDuration.Seconds())
	fmt.Printf("   🌳 Merkle root: %x\n", merkleRoot)

	if snap.Stats.ErrorCount > 0 {
		fmt.Printf("   ⚠️  Errors: %d\n", snap.Stats.ErrorCount)
	}

	return snap, err
}

// calculateSimpleMerkleRoot creates a simple merkle root without building full tree
func (s *Scanner) calculateSimpleMerkleRoot(files map[string]*snapshot.FileRecord) uint64 {
	if len(files) == 0 {
		return uint64(0)
	}

	// Create a sorted list of all file hashes for consistent merkle root
	var allHashes []string
	for _, record := range files {
		if record.Hash != "" && record.Hash != "ERROR" {
			allHashes = append(allHashes, record.Hash)
		}
		// Also include path for uniqueness
		allHashes = append(allHashes, record.Path)
	}

	// Sort for consistent ordering
	sort.Strings(allHashes)

	// Create a single hash from all file hashes
	hasher := xxhash.New()
	for _, hash := range allHashes {
		_, _ = hasher.Write([]byte(hash))
	}

	return xxhash.Sum64(hasher.Sum(nil))
}

// worker processes file jobs in parallel
func (s *Scanner) worker(id int, jobs <-chan FileJob, results chan<- FileResult, wg *sync.WaitGroup) {
	defer wg.Done()

	buffer := make([]byte, s.config.BufferSize)

	if s.config.Verbose {
		fmt.Printf("👷 Worker %d started\n", id)
	}

	for job := range jobs {
		result := s.processFile(job, buffer)
		results <- result

		// Update statistics
		if result.Error == nil {
			if result.Record.IsDir {
				atomic.AddInt64(&s.stats.DirsProcessed, 1)
			} else {
				atomic.AddInt64(&s.stats.FilesProcessed, 1)
				atomic.AddInt64(&s.stats.BytesProcessed, result.Record.Size)
			}
		} else {
			atomic.AddInt64(&s.stats.Errors, 1)
		}
	}

	if s.config.Verbose {
		fmt.Printf("👷 Worker %d finished\n", id)
	}
}

// processFile processes a single file and returns its record
func (s *Scanner) processFile(job FileJob, buffer []byte) FileResult {
	info, err := job.Info.Info()
	if err != nil {
		return FileResult{Error: fmt.Errorf("stat %s: %v", job.Path, err)}
	}

	record := &snapshot.FileRecord{
		Path:    job.Path,
		Size:    info.Size(),
		Mode:    info.Mode(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
	}

	// Get system-specific info (UID/GID on Unix)
	if stat := system.GetFileInfo(info); stat != nil {
		record.UID = stat.UID
		record.GID = stat.GID
	}

	// Calculate hash for regular files
	if info.Mode().IsRegular() && info.Size() > 0 {
		hash, err := s.hashFile(job.Path, buffer)
		if err != nil {
			if s.config.Verbose {
				fmt.Printf("⚠️  Hash error for %s: %v\n", job.Path, err)
			}
			record.Hash = "ERROR"
		} else {
			record.Hash = hash
		}
	}

	return FileResult{Record: record, Error: nil}
}

// hashFile calculates SHA256 hash efficiently using provided buffer
// HOT FILE, try to reduce file.read?
func (s *Scanner) hashFile(path string, buffer []byte) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := xxhash.New()
	for {
		n, err := file.Read(buffer)
		if n > 0 {
			_, _ = hash.Write(buffer[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// resultCollector collects results from workers
func (s *Scanner) resultCollector(results <-chan FileResult, files map[string]*snapshot.FileRecord, wg *sync.WaitGroup) {
	defer wg.Done()

	for result := range results {
		if result.Error != nil {
			if s.config.Verbose {
				fmt.Printf("⚠️  File error: %v\n", result.Error)
			}
			continue
		}
		files[result.Record.Path] = result.Record
	}
}

// progressMonitor displays scanning progress
func (s *Scanner) progressMonitor(ctx <-chan struct{}) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx:
			return
		case <-ticker.C:
			s.printProgress()
		}
	}
}

// printProgress prints current scanning progress
func (s *Scanner) printProgress() {
	s.stats.mutex.RLock()
	defer s.stats.mutex.RUnlock()

	now := time.Now()
	elapsed := now.Sub(s.stats.StartTime)

	files := atomic.LoadInt64(&s.stats.FilesProcessed)
	dirs := atomic.LoadInt64(&s.stats.DirsProcessed)
	bytes := atomic.LoadInt64(&s.stats.BytesProcessed)
	errors := atomic.LoadInt64(&s.stats.Errors)

	rate := float64(files+dirs) / elapsed.Seconds()

	fmt.Printf("📊 Progress: %d files, %d dirs, %s | %.0f items/sec | %d errors | %v elapsed\n",
		files, dirs, formatBytes(bytes), rate, errors, elapsed.Truncate(time.Second))
}

// ShouldIgnore checks if a path should be ignored
func (i *PathIgnorer) ShouldIgnore(path string) bool {
	// Check default patterns first
	for _, pattern := range i.defaults {
		if i.matchPattern(path, pattern) {
			return true
		}
	}

	// Check user patterns
	for _, pattern := range i.patterns {
		if i.matchPattern(path, pattern) {
			return true
		}
	}

	return false
}

// matchPattern performs pattern matching for ignore rules
func (i *PathIgnorer) matchPattern(path, pattern string) bool {
	// Handle different pattern types

	// Exact match
	if path == pattern {
		return true
	}

	// Directory name matching (e.g., ".cache" matches any .cache directory)
	pathParts := strings.Split(path, string(filepath.Separator))
	for _, part := range pathParts {
		if part == pattern {
			return true
		}
	}

	// Wildcard matching
	if strings.Contains(pattern, "*") {
		matched, _ := filepath.Match(pattern, filepath.Base(path))
		if matched {
			return true
		}
		// Also try matching the full path
		matched, _ = filepath.Match(pattern, path)
		if matched {
			return true
		}
	}

	// Prefix matching
	if strings.HasPrefix(path, pattern) {
		return true
	}

	// Suffix matching
	if strings.HasSuffix(path, pattern) {
		return true
	}

	// Contains matching (for things like "node_modules")
	if strings.Contains(path, pattern) {
		return true
	}

	return false
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
