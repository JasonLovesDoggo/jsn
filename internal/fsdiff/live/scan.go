package live

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	diff2 "pkg.jsn.cam/jsn/internal/fsdiff/diff"
	"pkg.jsn.cam/jsn/internal/fsdiff/scanner"
	"pkg.jsn.cam/jsn/internal/fsdiff/snapshot"
)

// baselineScanConfig returns a scanner config for the initial baseline scan (with progress)
func (s *Session) baselineScanConfig() *scanner.Config {
	return &scanner.Config{
		Workers:        s.config.Workers,
		Verbose:        s.config.Verbose,
		IgnorePatterns: s.config.IgnorePatterns,
	}
}

// createBaseline performs initial scan and saves as baseline
func (s *Session) createBaseline() error {
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

	// Pre-capture content for files in DiffDirs so we can compute diffs later
	if len(s.config.DiffDirs) > 0 {
		fmt.Printf("DiffDirs configured: %v\n", s.config.DiffDirs)
		captured := 0
		checked := 0
		for path, record := range snap.Files {
			if !record.IsDir && s.shouldComputeDiff(path) {
				checked++
				s.captureContent(path, record, nil)
				if s.store.ContentExists(record.Hash) {
					captured++
				}
			}
		}
		fmt.Printf("Pre-captured content for %d/%d files in diff directories\n", captured, checked)
	}

	return nil
}

// rebuildCurrentState applies all changes to baseline to get current state
func (s *Session) rebuildCurrentState() *snapshot.Snapshot {
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

// recordingLoop is the main recording loop
func (s *Session) recordingLoop(ctx context.Context) error {
	timer := time.NewTimer(s.config.Interval)
	defer timer.Stop()

	startTime := time.Now()

	// Initialize timing
	s.scanMu.Lock()
	s.lastScanTime = startTime
	s.nextScanTime = startTime.Add(s.config.Interval)
	s.scanMu.Unlock()

	fmt.Printf("Recording started. Press Ctrl+C to stop.\n")
	fmt.Printf("Scanning every %s\n", s.config.Interval)

	for {
		select {
		case <-ctx.Done():
			s.printSummary(startTime)
			return nil
		case newInterval := <-s.intervalCh:
			// Dynamic interval change
			timer.Stop()
			s.scanMu.Lock()
			s.config.Interval = newInterval
			s.nextScanTime = time.Now().Add(newInterval)
			s.scanMu.Unlock()
			timer = time.NewTimer(newInterval)
			fmt.Printf("Interval changed to %s\n", newInterval)
		case <-s.scanNowCh:
			// Manual scan trigger
			timer.Stop()
			s.scanMu.Lock()
			s.scanning = true
			s.scanMu.Unlock()
			if err := s.scan(ctx); err != nil {
				if ctx.Err() != nil {
					s.printSummary(startTime)
					return nil
				}
				fmt.Printf("Scan error: %v\n", err)
			}
			// Reset timer for next scan
			s.scanMu.Lock()
			s.scanning = false
			s.nextScanTime = time.Now().Add(s.config.Interval)
			s.scanMu.Unlock()
			timer = time.NewTimer(s.config.Interval)
		case <-timer.C:
			s.scanMu.Lock()
			s.scanning = true
			s.scanMu.Unlock()
			if err := s.scan(ctx); err != nil {
				if ctx.Err() != nil {
					s.printSummary(startTime)
					return nil
				}
				fmt.Printf("Scan error: %v\n", err)
			}
			// Reset timer for next scan
			s.scanMu.Lock()
			s.scanning = false
			s.nextScanTime = time.Now().Add(s.config.Interval)
			s.scanMu.Unlock()
			timer.Reset(s.config.Interval)
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

	// Increment scan ID and update timing
	s.scanMu.Lock()
	s.currentScanID++
	scanID := s.currentScanID
	s.lastScanTime = time.Now()
	s.scanMu.Unlock()

	scanStart := time.Now()
	if s.config.Verbose {
		fmt.Printf("[%s] Starting scan #%d...\n", scanStart.Format("15:04:05"), scanID)
	}

	// Use incremental mode - skip files that haven't changed since last scan
	scanConfig := &scanner.Config{
		Workers:          s.config.Workers,
		Verbose:          false,
		IgnorePatterns:   s.config.IgnorePatterns,
		PreviousSnapshot: s.current,
	}

	sc := scanner.New(scanConfig)

	// Store scanner ref for progress tracking
	s.scanMu.Lock()
	s.currentScanner = sc
	s.scanStartTime = scanStart
	s.scanMu.Unlock()

	newSnap, err := sc.ScanFilesystem(s.config.RootPath)

	// Clear scanner ref and update lastTotalFiles
	s.scanMu.Lock()
	if newSnap != nil {
		s.lastTotalFiles = int64(newSnap.Stats.FileCount + newSnap.Stats.DirCount)
	}
	s.currentScanner = nil
	s.scanMu.Unlock()

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
			ScanID:    scanID,
			Timestamp: now,
			Path:      path,
			Type:      ChangeAdded,
			Hash:      record.Hash,
			Size:      record.Size,
			Mode:      uint32(record.Mode),
		}
		newChanges = append(newChanges, change)

		if s.config.CaptureContent {
			s.captureContent(path, record, change)
		}
	}

	// Modified files
	for path, detail := range result.Modified {
		record := detail.NewRecord
		change := &Change{
			ScanID:    scanID,
			Timestamp: now,
			Path:      path,
			Type:      ChangeModified,
			Hash:      record.Hash,
			Size:      record.Size,
			Mode:      uint32(record.Mode),
			OldMode:   uint32(detail.OldRecord.Mode),
			OldSize:   detail.OldRecord.Size,
		}
		newChanges = append(newChanges, change)

		if s.config.CaptureContent {
			s.captureContent(path, record, change)
		}

		// Compute diff if in watched directory
		if s.shouldComputeDiff(path) && detail.OldRecord != nil && detail.OldRecord.Hash != "" {
			if s.config.Verbose {
				fmt.Printf("  Computing diff for %s (old hash: %s)\n", path, detail.OldRecord.Hash[:8])
			}
			change.Diff = s.computeDiff(path, detail.OldRecord.Hash)
			if change.Diff == "" && s.config.Verbose {
				fmt.Printf("  WARNING: Diff computation returned empty for %s\n", path)
			} else if s.config.Verbose {
				lineCount := strings.Count(change.Diff, "\n")
				fmt.Printf("  Generated diff with %d lines (%d bytes)\n", lineCount, len(change.Diff))
				preview := change.Diff
				if len(preview) > 200 {
					preview = preview[:200]
				}
				fmt.Printf("  Diff preview: %q\n", preview)
			}
		} else if s.config.Verbose && s.shouldComputeDiff(path) {
			fmt.Printf("  Skipping diff for %s: OldRecord=%v, OldHash=%q\n", path, detail.OldRecord != nil, func() string {
				if detail.OldRecord != nil {
					return detail.OldRecord.Hash
				}
				return ""
			}())
		}
	}

	// Deleted files
	for path := range result.Deleted {
		change := &Change{
			ScanID:    scanID,
			Timestamp: now,
			Path:      path,
			Type:      ChangeDeleted,
		}
		newChanges = append(newChanges, change)
	}

	// Count change types
	added, modified, deleted := 0, 0, 0
	for _, c := range newChanges {
		switch c.Type {
		case ChangeAdded:
			added++
		case ChangeModified:
			modified++
		case ChangeDeleted:
			deleted++
		}
	}

	// Create scan metadata
	scanEnd := time.Now()
	scan := &Scan{
		ID:        scanID,
		StartTime: scanStart,
		EndTime:   scanEnd,
		Duration:  scanEnd.Sub(scanStart).Milliseconds(),
		Added:     added,
		Modified:  modified,
		Deleted:   deleted,
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

		s.printChanges(newChanges)
		s.notifyCallbacks(newChanges)
	}

	// Track scan metadata
	s.scanMu.Lock()
	s.scans = append(s.scans, scan)
	s.scanMu.Unlock()

	if err := s.store.AppendScan(scan); err != nil {
		fmt.Printf("Warning: failed to save scan metadata: %v\n", err)
	}

	return nil
}

// GetScanProgress returns the current scan progress
func (s *Session) GetScanProgress() ScanProgress {
	s.scanMu.RLock()
	defer s.scanMu.RUnlock()

	if !s.scanning || s.currentScanner == nil {
		return ScanProgress{Scanning: false}
	}

	stats := s.currentScanner.GetStats()
	files := stats.GetFilesProcessed()
	elapsed := time.Since(s.scanStartTime).Seconds()

	rate := 0
	if elapsed > 0 {
		rate = int(float64(files) / elapsed)
	}

	percent := 0
	if s.lastTotalFiles > 0 {
		percent = int(float64(files) / float64(s.lastTotalFiles) * 100)
		if percent > 100 {
			percent = 99
		}
	}

	return ScanProgress{
		Scanning:       true,
		FilesProcessed: files,
		TotalFiles:     s.lastTotalFiles,
		Percent:        percent,
		Rate:           rate,
		StartedAt:      s.scanStartTime.UnixMilli(),
	}
}
