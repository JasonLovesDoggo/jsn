//go:generate go run github.com/a-h/templ/cmd/templ generate

package live

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	diff2 "pkg.jsn.cam/jsn/internal/fsdiff/diff"
	"pkg.jsn.cam/jsn/internal/fsdiff/scanner"
	"pkg.jsn.cam/jsn/internal/fsdiff/snapshot"
)

// ChangeType represents the type of filesystem change
type ChangeType string

const (
	ChangeAdded    ChangeType = "added"
	ChangeModified ChangeType = "modified"
	ChangeDeleted  ChangeType = "deleted"
)

// Change represents a single filesystem change
type Change struct {
	Timestamp  time.Time  `json:"ts"`
	Path       string     `json:"path"`
	Type       ChangeType `json:"type"`
	Hash       string     `json:"hash,omitempty"`
	Size       int64      `json:"size,omitempty"`
	Mode       uint32     `json:"mode,omitempty"`
	ContentKey string     `json:"content,omitempty"` // hash into content bucket
	BulkID     int        `json:"bulk,omitempty"`    // 0 if not part of bulk, >0 is bulk group ID
}

// Config holds session configuration
type Config struct {
	RootPath       string
	Interval       time.Duration
	DBPath         string
	Workers        int
	Verbose        bool
	IgnorePatterns []string
	CaptureContent bool   // capture text file contents
	WebAddr        string // address for web UI (empty = disabled)
}

// Session represents a recording session
type Session struct {
	config *Config
	store  *Store

	// In-memory state
	baseline *snapshot.Snapshot
	current  *snapshot.Snapshot
	changes  []*Change
	mu       sync.RWMutex

	// Scan configuration
	scanConfig *scanner.Config
	diffConfig *diff2.Config

	// Callbacks for web UI
	onChangeCallbacks []func([]*Change)
	callbackMu        sync.RWMutex
}

// NewSession creates a new recording session
func NewSession(config *Config) (*Session, error) {
	// Ensure data directory exists
	dir := filepath.Dir(config.DBPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	store, err := OpenStore(config.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}

	s := &Session{
		config:  config,
		store:   store,
		changes: make([]*Change, 0),
		scanConfig: &scanner.Config{
			Workers:        config.Workers,
			Verbose:        false, // Quiet for incremental scans
			IgnorePatterns: config.IgnorePatterns,
		},
		diffConfig: &diff2.Config{
			IgnorePatterns: config.IgnorePatterns,
			Verbose:        false,
		},
	}

	return s, nil
}

// baselineScanConfig returns a scanner config for the initial baseline scan (with progress)
func (s *Session) baselineScanConfig() *scanner.Config {
	return &scanner.Config{
		Workers:        s.config.Workers,
		Verbose:        s.config.Verbose, // Show progress during baseline
		IgnorePatterns: s.config.IgnorePatterns,
	}
}

// Resume attempts to resume an existing session
func (s *Session) Resume() error {
	// Load metadata
	meta, err := s.store.LoadMeta()
	if err != nil {
		return fmt.Errorf("load meta: %w", err)
	}

	// Verify root path matches (normalize paths for comparison)
	metaPath := filepath.Clean(meta.RootPath)
	configPath := filepath.Clean(s.config.RootPath)
	if metaPath != configPath {
		return fmt.Errorf("root path mismatch: session=%s, config=%s", meta.RootPath, s.config.RootPath)
	}

	// Load baseline
	baseline, err := s.store.LoadBaseline()
	if err != nil {
		return fmt.Errorf("load baseline: %w", err)
	}
	s.baseline = baseline

	// Load and replay changes to build current state
	changes, err := s.store.LoadChanges()
	if err != nil {
		return fmt.Errorf("load changes: %w", err)
	}
	s.changes = changes

	// Rebuild current state from baseline + changes
	s.current = s.rebuildCurrentState()

	if s.config.Verbose {
		fmt.Printf("Resumed session: %d changes since %s\n",
			len(s.changes), meta.StartTime.Format("15:04:05"))
	}

	return nil
}

// rebuildCurrentState applies all changes to baseline to get current state
func (s *Session) rebuildCurrentState() *snapshot.Snapshot {
	// Start with a copy of baseline
	current := &snapshot.Snapshot{
		SystemInfo: s.baseline.SystemInfo,
		Files:      make(map[string]*snapshot.FileRecord),
		Stats:      s.baseline.Stats,
	}

	// Copy all files from baseline
	for path, record := range s.baseline.Files {
		current.Files[path] = record
	}

	// Apply changes
	for _, change := range s.changes {
		switch change.Type {
		case ChangeAdded, ChangeModified:
			current.Files[change.Path] = &snapshot.FileRecord{
				Path: change.Path,
				Hash: change.Hash,
				Size: change.Size,
				Mode: os.FileMode(change.Mode),
			}
		case ChangeDeleted:
			delete(current.Files, change.Path)
		}
	}

	return current
}

// Start begins the recording session
func (s *Session) Start(ctx context.Context) error {
	// Set up signal handling
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	go func() {
		<-sigChan
		fmt.Println("\nStopping recording...")
		cancel()
	}()

	// Start web server immediately if configured
	if s.config.WebAddr != "" {
		webServer := NewWebServer(s, s.config.WebAddr)
		go func() {
			if err := webServer.Start(ctx); err != nil {
				fmt.Printf("Web server error: %v\n", err)
			}
		}()
	}

	// Check if resuming or starting fresh
	if s.store.HasBaseline() {
		if err := s.Resume(); err != nil {
			return fmt.Errorf("resume: %w", err)
		}
		fmt.Printf("Resuming recording session...\n")
	} else {
		// Create initial baseline
		if err := s.createBaseline(); err != nil {
			return fmt.Errorf("create baseline: %w", err)
		}
	}

	// Start recording loop
	return s.recordingLoop(ctx)
}

// createBaseline performs initial scan and saves as baseline
func (s *Session) createBaseline() error {
	// Use verbose config for baseline scan (shows progress)
	sc := scanner.New(s.baselineScanConfig())
	snap, err := sc.ScanFilesystem(s.config.RootPath)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}

	s.baseline = snap
	s.current = snap

	// Save to store
	meta := &Meta{
		StartTime: time.Now(),
		RootPath:  s.config.RootPath,
		Interval:  int(s.config.Interval.Seconds()),
	}
	if err := s.store.SaveMeta(meta); err != nil {
		return fmt.Errorf("save meta: %w", err)
	}

	if err := s.store.SaveBaseline(snap); err != nil {
		return fmt.Errorf("save baseline: %w", err)
	}

	return nil
}

// recordingLoop is the main recording loop
func (s *Session) recordingLoop(ctx context.Context) error {
	ticker := time.NewTicker(s.config.Interval)
	defer ticker.Stop()

	startTime := time.Now()

	fmt.Printf("Recording started. Press Ctrl+C to stop.\n")
	fmt.Printf("Scanning every %s\n", s.config.Interval)

	for {
		select {
		case <-ctx.Done():
			s.printSummary(startTime)
			return nil
		case <-ticker.C:
			if err := s.scan(ctx); err != nil {
				if ctx.Err() != nil {
					s.printSummary(startTime)
					return nil
				}
				fmt.Printf("Scan error: %v\n", err)
			}
		}
	}
}

// scan performs a single scan cycle
func (s *Session) scan(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	scanStart := time.Now()
	if s.config.Verbose {
		fmt.Printf("[%s] Starting scan...\n", scanStart.Format("15:04:05"))
	}

	// Use incremental mode - skip files that haven't changed since last scan
	scanConfig := &scanner.Config{
		Workers:          s.config.Workers,
		Verbose:          false, // Keep quiet during incremental
		IgnorePatterns:   s.config.IgnorePatterns,
		PreviousSnapshot: s.current, // INCREMENTAL MODE - skip unchanged files
	}

	// Scan current filesystem
	sc := scanner.New(scanConfig)
	newSnap, err := sc.ScanFilesystem(s.config.RootPath)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}

	scanDuration := time.Since(scanStart)
	if s.config.Verbose {
		fmt.Printf("[%s] Scan completed in %v (%d files)\n",
			time.Now().Format("15:04:05"), scanDuration.Round(time.Second), len(newSnap.Files))
	}

	// Diff against current state
	d := diff2.New(s.diffConfig)
	result := d.Compare(s.current, newSnap)

	// Process changes
	now := time.Now()
	var newChanges []*Change

	// Added files
	for path, record := range result.Added {
		change := &Change{
			Timestamp: now,
			Path:      path,
			Type:      ChangeAdded,
			Hash:      record.Hash,
			Size:      record.Size,
			Mode:      uint32(record.Mode),
		}
		newChanges = append(newChanges, change)

		// Capture content for text files
		if s.config.CaptureContent {
			s.captureContent(path, record, change)
		}
	}

	// Modified files
	for path, detail := range result.Modified {
		record := detail.NewRecord
		change := &Change{
			Timestamp: now,
			Path:      path,
			Type:      ChangeModified,
			Hash:      record.Hash,
			Size:      record.Size,
			Mode:      uint32(record.Mode),
		}
		newChanges = append(newChanges, change)

		// Capture content for text files
		if s.config.CaptureContent {
			s.captureContent(path, record, change)
		}
	}

	// Deleted files
	for path := range result.Deleted {
		change := &Change{
			Timestamp: now,
			Path:      path,
			Type:      ChangeDeleted,
		}
		newChanges = append(newChanges, change)
	}

	// Persist and update state
	if len(newChanges) > 0 {
		if err := s.store.AppendChanges(newChanges); err != nil {
			return fmt.Errorf("save changes: %w", err)
		}

		s.mu.Lock()
		s.changes = append(s.changes, newChanges...)
		s.current = newSnap
		s.mu.Unlock()

		// Print changes
		s.printChanges(newChanges)

		// Notify callbacks
		s.notifyCallbacks(newChanges)
	}

	return nil
}

// captureContent captures file content for text files
func (s *Session) captureContent(path string, record *snapshot.FileRecord, change *Change) {
	// Skip if too large (>1MB)
	if record.Size > 1024*1024 {
		return
	}

	// Skip if not a text file extension
	if !isTextFile(path) {
		return
	}

	// Skip if content already exists
	if s.store.ContentExists(record.Hash) {
		change.ContentKey = record.Hash
		return
	}

	// Read and save content
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}

	if err := s.store.SaveContent(record.Hash, content); err == nil {
		change.ContentKey = record.Hash
	}
}

// printChanges prints detected changes
func (s *Session) printChanges(changes []*Change) {
	for _, change := range changes {
		icon := getIcon(change.Type)
		fmt.Printf("%s %s %s\n", time.Now().Format("15:04:05"), icon, change.Path)
	}
}

// printSummary prints session summary
func (s *Session) printSummary(startTime time.Time) {
	duration := time.Since(startTime)

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("RECORDING SUMMARY")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Duration: %s\n", duration.Round(time.Second))
	fmt.Printf("Total changes: %d\n", len(s.changes))

	// Count by type
	added, modified, deleted := 0, 0, 0
	for _, c := range s.changes {
		switch c.Type {
		case ChangeAdded:
			added++
		case ChangeModified:
			modified++
		case ChangeDeleted:
			deleted++
		}
	}
	fmt.Printf("  Added:    %d\n", added)
	fmt.Printf("  Modified: %d\n", modified)
	fmt.Printf("  Deleted:  %d\n", deleted)

	// Show most changed paths
	pathCounts := make(map[string]int)
	for _, c := range s.changes {
		pathCounts[c.Path]++
	}

	fmt.Println("\nTop changed paths:")
	// Simple top 5
	type pathCount struct {
		path  string
		count int
	}
	var sorted []pathCount
	for path, count := range pathCounts {
		sorted = append(sorted, pathCount{path, count})
	}
	// Sort by count descending (simple bubble sort for small lists)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].count > sorted[i].count {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	for i := 0; i < len(sorted) && i < 5; i++ {
		fmt.Printf("  %d changes: %s\n", sorted[i].count, sorted[i].path)
	}

	fmt.Printf("\nSession saved to: %s\n", s.config.DBPath)
}

// OnChange registers a callback for new changes
func (s *Session) OnChange(cb func([]*Change)) {
	s.callbackMu.Lock()
	defer s.callbackMu.Unlock()
	s.onChangeCallbacks = append(s.onChangeCallbacks, cb)
}

// notifyCallbacks notifies all registered callbacks
func (s *Session) notifyCallbacks(changes []*Change) {
	s.callbackMu.RLock()
	defer s.callbackMu.RUnlock()
	for _, cb := range s.onChangeCallbacks {
		go cb(changes)
	}
}

// GetChanges returns a copy of all recorded changes
func (s *Session) GetChanges() []*Change {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Return a copy to prevent data races
	result := make([]*Change, len(s.changes))
	copy(result, s.changes)
	return result
}

// GetStore returns the underlying store
func (s *Session) GetStore() *Store {
	return s.store
}

// Close closes the session
func (s *Session) Close() error {
	return s.store.Close()
}

// Helper functions

func getIcon(t ChangeType) string {
	switch t {
	case ChangeAdded:
		return "+"
	case ChangeModified:
		return "~"
	case ChangeDeleted:
		return "-"
	default:
		return "?"
	}
}

func isTextFile(path string) bool {
	textExts := []string{
		".txt", ".md", ".json", ".yaml", ".yml", ".toml",
		".go", ".py", ".js", ".ts", ".jsx", ".tsx",
		".html", ".css", ".scss", ".less",
		".sh", ".bash", ".zsh", ".fish",
		".c", ".h", ".cpp", ".hpp", ".rs",
		".java", ".kt", ".scala",
		".rb", ".php", ".pl", ".lua",
		".sql", ".xml", ".csv",
		".conf", ".cfg", ".ini", ".env",
		".log", ".gitignore", ".dockerignore",
	}

	ext := strings.ToLower(filepath.Ext(path))
	for _, textExt := range textExts {
		if ext == textExt {
			return true
		}
	}

	// Also check common config files without extensions
	base := filepath.Base(path)
	configFiles := []string{
		"Makefile", "Dockerfile", "Vagrantfile",
		".bashrc", ".zshrc", ".profile",
		"passwd", "shadow", "group", "hosts",
		"fstab", "crontab", "sudoers",
	}
	for _, cf := range configFiles {
		if base == cf {
			return true
		}
	}

	return false
}
