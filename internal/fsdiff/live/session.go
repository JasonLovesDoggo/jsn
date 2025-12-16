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

	// Scan timing and tracking
	currentScanID int
	scans         []*Scan
	lastScanTime  time.Time
	nextScanTime  time.Time
	intervalCh    chan time.Duration
	scanNowCh     chan struct{}
	scanning      bool
	scanMu        sync.RWMutex

	// Progress tracking
	currentScanner *scanner.Scanner
	scanStartTime  time.Time
	lastTotalFiles int64

	// Callbacks for web UI
	onChangeCallbacks []func([]*Change)
	callbackMu        sync.RWMutex
}

// NewSession creates a new recording session
func NewSession(config *Config) (*Session, error) {
	dir := filepath.Dir(config.DBPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	store, err := OpenStore(config.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}

	s := &Session{
		config:     config,
		store:      store,
		changes:    make([]*Change, 0),
		scans:      make([]*Scan, 0),
		intervalCh: make(chan time.Duration, 1),
		scanNowCh:  make(chan struct{}, 1),
		scanConfig: &scanner.Config{
			Workers:        config.Workers,
			Verbose:        false,
			IgnorePatterns: config.IgnorePatterns,
		},
		diffConfig: &diff2.Config{
			IgnorePatterns: config.IgnorePatterns,
			Verbose:        false,
		},
	}

	return s, nil
}

// Resume attempts to resume an existing session
func (s *Session) Resume() error {
	meta, err := s.store.LoadMeta()
	if err != nil {
		return fmt.Errorf("load meta: %w", err)
	}

	// Verify root path matches
	metaPath := filepath.Clean(meta.RootPath)
	configPath := filepath.Clean(s.config.RootPath)
	if metaPath != configPath {
		return fmt.Errorf("root path mismatch: session=%s, config=%s", meta.RootPath, s.config.RootPath)
	}

	baseline, err := s.store.LoadBaseline()
	if err != nil {
		return fmt.Errorf("load baseline: %w", err)
	}
	s.baseline = baseline

	changes, err := s.store.LoadChanges()
	if err != nil {
		return fmt.Errorf("load changes: %w", err)
	}
	s.changes = changes

	s.current = s.rebuildCurrentState()

	if s.config.Verbose {
		fmt.Printf("Resumed session: %d changes since %s\n",
			len(s.changes), meta.StartTime.Format("15:04:05"))
	}

	return nil
}

// Start begins the recording session
func (s *Session) Start(ctx context.Context) error {
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

	// Start web server if configured
	if s.config.WebAddr != "" {
		webServer := NewWebServer(s, s.config.WebAddr)
		go func() {
			if err := webServer.Start(ctx); err != nil {
				fmt.Printf("Web server error: %v\n", err)
			}
		}()
	}

	// Resume or create baseline
	if s.store.HasBaseline() {
		if err := s.Resume(); err != nil {
			return fmt.Errorf("resume: %w", err)
		}
		fmt.Printf("Resuming recording session...\n")
	} else {
		if err := s.createBaseline(); err != nil {
			return fmt.Errorf("create baseline: %w", err)
		}
	}

	return s.recordingLoop(ctx)
}

// Close closes the session
func (s *Session) Close() error {
	return s.store.Close()
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
	result := make([]*Change, len(s.changes))
	copy(result, s.changes)
	return result
}

// GetScans returns a copy of all recorded scans
func (s *Session) GetScans() []*Scan {
	s.scanMu.RLock()
	defer s.scanMu.RUnlock()
	result := make([]*Scan, len(s.scans))
	copy(result, s.scans)
	return result
}

// GetScanTiming returns current scan timing info
func (s *Session) GetScanTiming() (lastScan, nextScan time.Time, interval time.Duration) {
	s.scanMu.RLock()
	defer s.scanMu.RUnlock()
	return s.lastScanTime, s.nextScanTime, s.config.Interval
}

// SetInterval updates the scan interval dynamically
func (s *Session) SetInterval(d time.Duration) {
	select {
	case s.intervalCh <- d:
	default:
	}
}

// TriggerScan triggers an immediate scan
func (s *Session) TriggerScan() {
	select {
	case s.scanNowCh <- struct{}{}:
	default:
	}
}

// IsScanning returns true if a scan is currently in progress
func (s *Session) IsScanning() bool {
	s.scanMu.RLock()
	defer s.scanMu.RUnlock()
	return s.scanning
}

// GetIgnorePatterns returns the current ignore patterns
func (s *Session) GetIgnorePatterns() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, len(s.config.IgnorePatterns))
	copy(result, s.config.IgnorePatterns)
	return result
}

// SetIgnorePatterns updates ignore patterns and filters existing changes
func (s *Session) SetIgnorePatterns(patterns []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config.IgnorePatterns = patterns
	s.scanConfig.IgnorePatterns = patterns
	s.diffConfig.IgnorePatterns = patterns

	var filtered []*Change
	for _, c := range s.changes {
		if !s.matchesIgnorePattern(c.Path) {
			filtered = append(filtered, c)
		}
	}
	s.changes = filtered
}

// matchesIgnorePattern checks if path matches any ignore pattern
func (s *Session) matchesIgnorePattern(path string) bool {
	for _, pattern := range s.config.IgnorePatterns {
		if strings.HasPrefix(path, pattern) {
			return true
		}
	}
	return false
}

// GetStore returns the underlying store
func (s *Session) GetStore() *Store {
	return s.store
}
