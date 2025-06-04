package data

import (
	"fmt"
	"io/fs"
	"time"

	"pkg.jsn.cam/jsn/cmd/fsdiff/internal/merkle"

	"pkg.jsn.cam/jsn/cmd/fsdiff/internal/system"
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

// GetPath returns the file path
func (f *FileRecord) GetPath() string {
	return f.Path
}

// GetHash returns the file hash
func (f *FileRecord) GetHash() string {
	return f.Hash
}

// GetSize returns the file size
func (f *FileRecord) GetSize() int64 {
	return f.Size
}

// GetMode returns the file mode
func (f *FileRecord) GetMode() fs.FileMode {
	return f.Mode
}

// GetModTime returns the file modification time
func (f *FileRecord) GetModTime() time.Time {
	return f.ModTime
}

// GetUID returns the file UID
func (f *FileRecord) GetUID() int {
	return f.UID
}

// GetGID returns the file GID
func (f *FileRecord) GetGID() int {
	return f.GID
}

// ScanStats contains statistics about the filesystem scan
type ScanStats struct {
	FileCount    int           `json:"file_count"`
	DirCount     int           `json:"dir_count"`
	TotalSize    int64         `json:"total_size"`
	ErrorCount   int           `json:"error_count"`
	ScanDuration time.Duration `json:"scan_duration"`
}

// Snapshot represents a complete filesystem snapshot
type Snapshot struct {
	SystemInfo system.SystemInfo      `json:"system_info"`
	Files      map[string]*FileRecord `json:"files"`
	MerkleRoot uint64                 `json:"merkle_root"`
	Tree       *merkle.Tree           `json:"-"` // Don't serialize the full tree
	Stats      ScanStats              `json:"stats"`
	Version    string                 `json:"version"`
}

// SnapshotHeader contains metadata for quick snapshot inspection
type SnapshotHeader struct {
	Version    string            `json:"version"`
	SystemInfo system.SystemInfo `json:"system_info"`
	Stats      ScanStats         `json:"stats"`
	MerkleRoot uint64            `json:"merkle_root"`
	Created    time.Time         `json:"created"`
}

const SnapshotVersion = "2.0.0"

// Validate performs basic validation on a snapshot
func (s *Snapshot) Validate() error {
	if s.Version == "" {
		return fmt.Errorf("missing snapshot version")
	}

	if s.SystemInfo.Hostname == "" {
		return fmt.Errorf("missing system hostname")
	}

	if len(s.Files) == 0 {
		return fmt.Errorf("snapshot contains no files")
	}

	if s.Stats.FileCount == 0 && s.Stats.DirCount == 0 {
		return fmt.Errorf("invalid statistics: no files or directories")
	}

	// Verify file count matches
	actualFiles := 0
	actualDirs := 0
	for _, record := range s.Files {
		if record.IsDir {
			actualDirs++
		} else {
			actualFiles++
		}
	}

	if actualFiles != s.Stats.FileCount {
		return fmt.Errorf("file count mismatch: expected %d, got %d",
			s.Stats.FileCount, actualFiles)
	}

	if actualDirs != s.Stats.DirCount {
		return fmt.Errorf("directory count mismatch: expected %d, got %d",
			s.Stats.DirCount, actualDirs)
	}

	return nil
}

// GetFileRecord retrieves a file record by path
func (s *Snapshot) GetFileRecord(path string) (*FileRecord, bool) {
	record, exists := s.Files[path]
	return record, exists
}

// GetFilesByPattern returns files matching a pattern
func (s *Snapshot) GetFilesByPattern(pattern string) []*FileRecord {
	var matches []*FileRecord

	for _, record := range s.Files {
		// Simple pattern matching - you could use filepath.Match for more complex patterns
		if matchesPattern(record.Path, pattern) {
			matches = append(matches, record)
		}
	}

	return matches
}

// matchesPattern performs simple pattern matching
func matchesPattern(path, pattern string) bool {
	// For now, just do simple substring matching
	// Could be enhanced with proper glob pattern matching
	if pattern == "*" {
		return true
	}

	// Check if pattern is contained in path
	return contains(path, pattern)
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsAt(s, substr))))
}

// containsAt checks if substr appears anywhere in s
func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Summary returns a summary of the snapshot
func (s *Snapshot) Summary() string {
	return fmt.Sprintf("Snapshot: %s@%s (%d files, %d dirs, %s, scan took %v)",
		s.SystemInfo.Hostname,
		s.SystemInfo.Timestamp.Format("2006-01-02 15:04:05"),
		s.Stats.FileCount,
		s.Stats.DirCount,
		formatBytes(s.Stats.TotalSize),
		s.Stats.ScanDuration.Truncate(time.Second))
}

// Clone creates a deep copy of the snapshot (excluding Tree)
func (s *Snapshot) Clone() *Snapshot {
	clone := &Snapshot{
		SystemInfo: s.SystemInfo,
		Files:      make(map[string]*FileRecord),
		MerkleRoot: s.MerkleRoot,
		Stats:      s.Stats,
		Version:    s.Version,
	}

	// Deep copy files map
	for path, record := range s.Files {
		clone.Files[path] = &FileRecord{
			Path:    record.Path,
			Hash:    record.Hash,
			Size:    record.Size,
			Mode:    record.Mode,
			ModTime: record.ModTime,
			IsDir:   record.IsDir,
			UID:     record.UID,
			GID:     record.GID,
		}
	}

	return clone
}

// FilterFiles creates a new snapshot with only files matching the filter function
func (s *Snapshot) FilterFiles(filter func(*FileRecord) bool) *Snapshot {
	filtered := s.Clone()
	filtered.Files = make(map[string]*FileRecord)

	fileCount := 0
	dirCount := 0
	var totalSize int64

	for path, record := range s.Files {
		if filter(record) {
			filtered.Files[path] = record
			if record.IsDir {
				dirCount++
			} else {
				fileCount++
				totalSize += record.Size
			}
		}
	}

	// Update stats
	filtered.Stats.FileCount = fileCount
	filtered.Stats.DirCount = dirCount
	filtered.Stats.TotalSize = totalSize

	return filtered
}

// MergeWith merges another snapshot into this one (for incremental updates)
func (s *Snapshot) MergeWith(other *Snapshot) {
	for path, record := range other.Files {
		s.Files[path] = record
	}

	// Update statistics (simple addition - might not be accurate)
	s.Stats.FileCount += other.Stats.FileCount
	s.Stats.DirCount += other.Stats.DirCount
	s.Stats.TotalSize += other.Stats.TotalSize
}

// GetDirectoryTree returns a hierarchical view of directories
func (s *Snapshot) GetDirectoryTree() map[string][]string {
	tree := make(map[string][]string)

	for path, record := range s.Files {
		if record.IsDir {
			// Extract parent directory
			parent := getParentDir(path)
			if parent != "" {
				if tree[parent] == nil {
					tree[parent] = make([]string, 0)
				}
				tree[parent] = append(tree[parent], path)
			}
		}
	}

	return tree
}

// getParentDir extracts the parent directory from a path
func getParentDir(path string) string {
	if path == "" || path == "/" {
		return ""
	}

	// Remove trailing slash
	if path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}

	// Find last slash
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			if i == 0 {
				return "/"
			}
			return path[:i]
		}
	}

	return ""
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
