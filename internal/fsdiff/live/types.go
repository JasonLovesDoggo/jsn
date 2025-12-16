package live

import "time"

// ChangeType represents the type of filesystem change
type ChangeType string

const (
	ChangeAdded    ChangeType = "added"
	ChangeModified ChangeType = "modified"
	ChangeDeleted  ChangeType = "deleted"
)

// Change represents a single filesystem change
type Change struct {
	ScanID     int        `json:"scanId,omitempty"` // which scan detected this change
	Timestamp  time.Time  `json:"ts"`
	Path       string     `json:"path"`
	Type       ChangeType `json:"type"`
	Hash       string     `json:"hash,omitempty"`
	Size       int64      `json:"size,omitempty"`
	Mode       uint32     `json:"mode,omitempty"`
	OldMode    uint32     `json:"oldMode,omitempty"` // previous mode for modified files
	OldSize    int64      `json:"oldSize,omitempty"` // previous size for modified files
	ContentKey string     `json:"content,omitempty"` // hash into content bucket
	BulkID     int        `json:"bulk,omitempty"`    // 0 if not part of bulk, >0 is bulk group ID
	Diff       string     `json:"diff,omitempty"`    // unified diff for modified files
}

// Scan represents metadata for a single scan cycle
type Scan struct {
	ID        int       `json:"id"`
	StartTime time.Time `json:"start"`
	EndTime   time.Time `json:"end"`
	Duration  int64     `json:"durationMs"`
	Added     int       `json:"added"`
	Modified  int       `json:"modified"`
	Deleted   int       `json:"deleted"`
}

// ScanProgress represents the progress of an ongoing scan
type ScanProgress struct {
	Scanning       bool  `json:"scanning"`
	FilesProcessed int64 `json:"filesProcessed"`
	TotalFiles     int64 `json:"totalFiles"`
	Percent        int   `json:"percent"`
	Rate           int   `json:"rate"`
	StartedAt      int64 `json:"startedAt"`
}

// Config holds session configuration
type Config struct {
	RootPath       string
	Interval       time.Duration
	Workers        int
	DBPath         string
	Verbose        bool
	IgnorePatterns []string
	CaptureContent bool     // capture text file contents
	WebAddr        string   // address for web UI (empty = disabled)
	DiffDirs       []string // directories to compute diffs for (must be under RootPath)
}
