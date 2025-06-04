package main

import (
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// FileRecord represents a single file's metadata and hash
type FileRecord struct {
	Path    string      `json:"path"`
	Hash    string      `json:"hash"`
	Size    int64       `json:"size"`
	Mode    fs.FileMode `json:"mode"`
	ModTime time.Time   `json:"mod_time"`
	IsDir   bool        `json:"is_dir"`
	UID     int         `json:"uid,omitempty"`
	GID     int         `json:"gid,omitempty"`
}

// SystemInfo contains metadata about the system when snapshot was taken
type SystemInfo struct {
	Hostname     string        `json:"hostname"`
	OS           string        `json:"os"`
	Arch         string        `json:"arch"`
	Distro       string        `json:"distro"`
	KernelVer    string        `json:"kernel_version"`
	Timestamp    time.Time     `json:"timestamp"`
	ScanRoot     string        `json:"scan_root"`
	TotalFiles   int           `json:"total_files"`
	TotalDirs    int           `json:"total_dirs"`
	ScanDuration time.Duration `json:"scan_duration"`
}

// Snapshot represents a complete filesystem snapshot
type Snapshot struct {
	SystemInfo SystemInfo             `json:"system_info"`
	Files      map[string]*FileRecord `json:"files"`
}

// DiffResult represents the comparison between two snapshots
type DiffResult struct {
	BaselineInfo SystemInfo             `json:"baseline_info"`
	CurrentInfo  SystemInfo             `json:"current_info"`
	Added        map[string]*FileRecord `json:"added"`
	Modified     map[string]*FileRecord `json:"modified"`
	Deleted      map[string]*FileRecord `json:"deleted"`
	Summary      DiffSummary            `json:"summary"`
}

type DiffSummary struct {
	AddedCount    int `json:"added_count"`
	ModifiedCount int `json:"modified_count"`
	DeletedCount  int `json:"deleted_count"`
	TotalChanges  int `json:"total_changes"`
}

// Default exclusions for common system directories that change frequently
var defaultExclusions = []string{
	"/proc",
	"/sys",
	"/dev",
	"/tmp",
	"/var/tmp",
	"/run",
	"/var/run",
	"/var/log",
	"/var/cache",
	"/var/lib/dhcp",
	"/home/*/.cache",
	"/home/*/.local/share/Trash",
	"/home/*/.mozilla/firefox/*/Cache",
	"/home/*/.config/google-chrome/*/Cache",
}

// shouldExclude checks if a path should be excluded from scanning
func shouldExclude(path string) bool {
	for _, exclusion := range defaultExclusions {
		// Simple wildcard matching
		if strings.Contains(exclusion, "*") {
			pattern := strings.Replace(exclusion, "*", "", -1)
			if strings.Contains(path, pattern) {
				return true
			}
		} else if strings.HasPrefix(path, exclusion) {
			return true
		}
	}
	return false
}

// hashFile calculates SHA256 hash of a file
func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// getSystemInfo gathers system metadata
func getSystemInfo(scanRoot string) SystemInfo {
	hostname, _ := os.Hostname()

	// Try to detect Linux distro
	distro := "unknown"
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				distro = strings.Trim(strings.Split(line, "=")[1], "\"")
				break
			}
		}
	}

	// Try to get kernel version
	kernelVer := "unknown"
	if data, err := os.ReadFile("/proc/version"); err == nil {
		kernelVer = strings.Split(string(data), " ")[2]
	}

	return SystemInfo{
		Hostname:  hostname,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Distro:    distro,
		KernelVer: kernelVer,
		Timestamp: time.Now(),
		ScanRoot:  scanRoot,
	}
}

// createSnapshot scans the filesystem and creates a snapshot
func createSnapshot(rootPath string) (*Snapshot, error) {
	fmt.Printf("Creating filesystem snapshot of %s...\n", rootPath)
	startTime := time.Now()

	systemInfo := getSystemInfo(rootPath)
	files := make(map[string]*FileRecord)

	totalFiles := 0
	totalDirs := 0

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Printf("Warning: Cannot access %s: %v\n", path, err)
			return nil // Continue scanning
		}

		// Check exclusions
		if shouldExclude(path) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			fmt.Printf("Warning: Cannot get info for %s: %v\n", path, err)
			return nil
		}

		record := &FileRecord{
			Path:    path,
			Size:    info.Size(),
			Mode:    info.Mode(),
			ModTime: info.ModTime(),
			IsDir:   info.IsDir(),
		}

		// Calculate hash for regular files only
		if info.Mode().IsRegular() {
			hash, err := hashFile(path)
			if err != nil {
				fmt.Printf("Warning: Cannot hash %s: %v\n", path, err)
				record.Hash = "ERROR"
			} else {
				record.Hash = hash
			}
			totalFiles++
		} else if info.IsDir() {
			totalDirs++
		}

		files[path] = record

		// Progress indicator
		if (totalFiles+totalDirs)%10000 == 0 {
			fmt.Printf("Processed %d files and directories...\n", totalFiles+totalDirs)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan filesystem: %v", err)
	}

	duration := time.Since(startTime)
	systemInfo.TotalFiles = totalFiles
	systemInfo.TotalDirs = totalDirs
	systemInfo.ScanDuration = duration

	fmt.Printf("Scan complete: %d files, %d directories in %v\n",
		totalFiles, totalDirs, duration)

	return &Snapshot{
		SystemInfo: systemInfo,
		Files:      files,
	}, nil
}

// saveSnapshot saves a snapshot to disk using gob encoding
func saveSnapshot(snapshot *Snapshot, filename string) error {
	fmt.Printf("Saving snapshot to %s...\n", filename)

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create snapshot file: %v", err)
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(snapshot); err != nil {
		return fmt.Errorf("failed to encode snapshot: %v", err)
	}

	// Get file size for info
	stat, _ := file.Stat()
	fmt.Printf("Snapshot saved (%d bytes)\n", stat.Size())

	return nil
}

// loadSnapshot loads a snapshot from disk
func loadSnapshot(filename string) (*Snapshot, error) {
	fmt.Printf("Loading snapshot from %s...\n", filename)

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open snapshot file: %v", err)
	}
	defer file.Close()

	decoder := gob.NewDecoder(file)
	var snapshot Snapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("failed to decode snapshot: %v", err)
	}

	fmt.Printf("Snapshot loaded: %s (%s) - %d files, %d dirs\n",
		snapshot.SystemInfo.Hostname,
		snapshot.SystemInfo.Timestamp.Format("2006-01-02 15:04:05"),
		snapshot.SystemInfo.TotalFiles,
		snapshot.SystemInfo.TotalDirs)

	return &snapshot, nil
}

// compareSnapshots compares two snapshots and returns differences
func compareSnapshots(baseline, current *Snapshot) *DiffResult {
	fmt.Println("Comparing snapshots...")

	result := &DiffResult{
		BaselineInfo: baseline.SystemInfo,
		CurrentInfo:  current.SystemInfo,
		Added:        make(map[string]*FileRecord),
		Modified:     make(map[string]*FileRecord),
		Deleted:      make(map[string]*FileRecord),
	}

	// Find added and modified files
	for path, currentFile := range current.Files {
		baselineFile, exists := baseline.Files[path]

		if !exists {
			// File was added
			result.Added[path] = currentFile
		} else if !filesEqual(baselineFile, currentFile) {
			// File was modified
			result.Modified[path] = currentFile
		}
	}

	// Find deleted files
	for path, baselineFile := range baseline.Files {
		if _, exists := current.Files[path]; !exists {
			result.Deleted[path] = baselineFile
		}
	}

	// Calculate summary
	result.Summary = DiffSummary{
		AddedCount:    len(result.Added),
		ModifiedCount: len(result.Modified),
		DeletedCount:  len(result.Deleted),
		TotalChanges:  len(result.Added) + len(result.Modified) + len(result.Deleted),
	}

	fmt.Printf("Comparison complete: %d changes (%d added, %d modified, %d deleted)\n",
		result.Summary.TotalChanges,
		result.Summary.AddedCount,
		result.Summary.ModifiedCount,
		result.Summary.DeletedCount)

	return result
}

// filesEqual compares two file records for equality
func filesEqual(a, b *FileRecord) bool {
	if a.IsDir && b.IsDir {
		// For directories, compare metadata only
		return a.Mode == b.Mode && a.ModTime.Equal(b.ModTime)
	}

	if a.IsDir != b.IsDir {
		return false
	}

	// For files, compare hash, size, and mode
	return a.Hash == b.Hash && a.Size == b.Size && a.Mode == b.Mode
}

// printDiffSummary prints a human-readable summary of the diff
func printDiffSummary(diff *DiffResult) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("FILESYSTEM DIFF REPORT")
	fmt.Println(strings.Repeat("=", 60))

	fmt.Printf("Baseline: %s (%s) on %s\n",
		diff.BaselineInfo.Hostname,
		diff.BaselineInfo.Distro,
		diff.BaselineInfo.Timestamp.Format("2006-01-02 15:04:05"))

	fmt.Printf("Current:  %s (%s) on %s\n\n",
		diff.CurrentInfo.Hostname,
		diff.CurrentInfo.Distro,
		diff.CurrentInfo.Timestamp.Format("2006-01-02 15:04:05"))

	fmt.Printf("SUMMARY:\n")
	fmt.Printf("  Added:    %d files/directories\n", diff.Summary.AddedCount)
	fmt.Printf("  Modified: %d files/directories\n", diff.Summary.ModifiedCount)
	fmt.Printf("  Deleted:  %d files/directories\n", diff.Summary.DeletedCount)
	fmt.Printf("  Total:    %d changes\n\n", diff.Summary.TotalChanges)

	if diff.Summary.TotalChanges == 0 {
		fmt.Println("✅ No changes detected!")
		return
	}

	// Show some examples of changes
	if len(diff.Added) > 0 {
		fmt.Printf("ADDED FILES (showing first 10):\n")
		count := 0
		for path := range diff.Added {
			if count >= 100 {
				fmt.Printf("  ... and %d more\n", len(diff.Added)-10)
				break
			}
			fmt.Printf("  + %s\n", path)
			count++
		}
		fmt.Println()
	}

	if len(diff.Modified) > 0 {
		fmt.Printf("MODIFIED FILES (showing first 10):\n")
		count := 0
		for path := range diff.Modified {
			if count >= 1000 {
				fmt.Printf("  ... and %d more\n", len(diff.Modified)-10)
				break
			}
			fmt.Printf("  ~ %s\n", path)
			count++
		}
		fmt.Println()
	}

	if len(diff.Deleted) > 0 {
		fmt.Printf("DELETED FILES (showing first 10):\n")
		count := 0
		for path := range diff.Deleted {
			if count >= 100 {
				fmt.Printf("  ... and %d more\n", len(diff.Deleted)-10)
				break
			}
			fmt.Printf("  - %s\n", path)
			count++
		}
		fmt.Println()
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Filesystem Diff Tool v1.0")
		fmt.Println("Usage:")
		fmt.Println("  Create snapshot:    ./fsdiff snapshot <root_path> <output_file>")
		fmt.Println("  Compare snapshots:  ./fsdiff compare <baseline_file> <current_file>")
		fmt.Println("  Live comparison:    ./fsdiff live <baseline_file> <root_path>")
		fmt.Println("\nExamples:")
		fmt.Println("  ./fsdiff snapshot / baseline.snap")
		fmt.Println("  ./fsdiff compare baseline.snap current.snap")
		fmt.Println("  ./fsdiff live baseline.snap /")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "snapshot":
		if len(os.Args) != 4 {
			fmt.Println("Usage: ./fsdiff snapshot <root_path> <output_file>")
			os.Exit(1)
		}

		rootPath := os.Args[2]
		outputFile := os.Args[3]

		snapshot, err := createSnapshot(rootPath)
		if err != nil {
			fmt.Printf("Error creating snapshot: %v\n", err)
			os.Exit(1)
		}

		if err := saveSnapshot(snapshot, outputFile); err != nil {
			fmt.Printf("Error saving snapshot: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ Snapshot created successfully: %s\n", outputFile)

	case "compare":
		if len(os.Args) != 4 {
			fmt.Println("Usage: ./fsdiff compare <baseline_file> <current_file>")
			os.Exit(1)
		}

		baselineFile := os.Args[2]
		currentFile := os.Args[3]

		baseline, err := loadSnapshot(baselineFile)
		if err != nil {
			fmt.Printf("Error loading baseline snapshot: %v\n", err)
			os.Exit(1)
		}

		current, err := loadSnapshot(currentFile)
		if err != nil {
			fmt.Printf("Error loading current snapshot: %v\n", err)
			os.Exit(1)
		}

		diff := compareSnapshots(baseline, current)
		printDiffSummary(diff)

	case "live":
		if len(os.Args) != 4 {
			fmt.Println("Usage: ./fsdiff live <baseline_file> <root_path>")
			os.Exit(1)
		}

		baselineFile := os.Args[2]
		rootPath := os.Args[3]

		baseline, err := loadSnapshot(baselineFile)
		if err != nil {
			fmt.Printf("Error loading baseline snapshot: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Creating current snapshot for comparison...")
		current, err := createSnapshot(rootPath)
		if err != nil {
			fmt.Printf("Error creating current snapshot: %v\n", err)
			os.Exit(1)
		}

		diff := compareSnapshots(baseline, current)
		printDiffSummary(diff)

	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Use 'snapshot', 'compare', or 'live'")
		os.Exit(1)
	}
}
