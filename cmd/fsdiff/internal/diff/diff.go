package diff

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"pkg.jsn.cam/jsn/cmd/fsdiff/internal/snapshot"
)

// Config holds diff configuration
type Config struct {
	IgnorePatterns []string
	Verbose        bool
	ShowHashes     bool
	OnlyChanges    bool
}

// Differ handles comparing snapshots
type Differ struct {
	config  *Config
	ignorer *PathIgnorer
}

// Result represents the comparison between two snapshots
type Result struct {
	Baseline  *snapshot.Snapshot              `json:"baseline"`
	Current   *snapshot.Snapshot              `json:"current"`
	Added     map[string]*snapshot.FileRecord `json:"added"`
	Modified  map[string]*ChangeDetail        `json:"modified"`
	Deleted   map[string]*snapshot.FileRecord `json:"deleted"`
	Summary   Summary                         `json:"summary"`
	Generated time.Time                       `json:"generated"`
}

// ChangeDetail represents details about a modified file
type ChangeDetail struct {
	OldRecord *snapshot.FileRecord `json:"old_record"`
	NewRecord *snapshot.FileRecord `json:"new_record"`
	Changes   []string             `json:"changes"`
}

// Summary contains summary statistics
type Summary struct {
	AddedCount     int           `json:"added_count"`
	ModifiedCount  int           `json:"modified_count"`
	DeletedCount   int           `json:"deleted_count"`
	TotalChanges   int           `json:"total_changes"`
	AddedSize      int64         `json:"added_size"`
	DeletedSize    int64         `json:"deleted_size"`
	SizeDiff       int64         `json:"size_diff"`
	ComparisonTime time.Duration `json:"comparison_time"`
}

// PathIgnorer handles ignore pattern matching for diffs
type PathIgnorer struct {
	patterns []string
}

// ChangeType represents the type of change
type ChangeType string

const (
	ChangeAdded    ChangeType = "added"
	ChangeModified ChangeType = "modified"
	ChangeDeleted  ChangeType = "deleted"
)

// New creates a new differ
func New(config *Config) *Differ {
	if config == nil {
		config = &Config{}
	}

	return &Differ{
		config: config,
		ignorer: &PathIgnorer{
			patterns: config.IgnorePatterns,
		},
	}
}

// Compare compares two snapshots and returns the differences
func (d *Differ) Compare(baseline, current *snapshot.Snapshot) *Result {
	startTime := time.Now()

	if d.config.Verbose {
		fmt.Printf("🔍 Comparing snapshots...\n")
		fmt.Printf("   Baseline: %d files (%s)\n",
			baseline.Stats.FileCount, baseline.SystemInfo.Timestamp.Format("2006-01-02 15:04:05"))
		fmt.Printf("   Current:  %d files (%s)\n",
			current.Stats.FileCount, current.SystemInfo.Timestamp.Format("2006-01-02 15:04:05"))
	}

	result := &Result{
		Baseline:  baseline,
		Current:   current,
		Added:     make(map[string]*snapshot.FileRecord),
		Modified:  make(map[string]*ChangeDetail),
		Deleted:   make(map[string]*snapshot.FileRecord),
		Generated: time.Now(),
	}

	// Use Merkle tree comparison for efficiency if available
	if baseline.Tree != nil && current.Tree != nil {
		d.compareMerkleTrees(baseline, current, result)
	} else {
		d.compareBruteForce(baseline, current, result)
	}

	// Calculate summary
	result.Summary = d.calculateSummary(result, time.Since(startTime))

	if d.config.Verbose {
		fmt.Printf("✅ Comparison completed in %v\n", time.Since(startTime))
		fmt.Printf("   Changes: %d added, %d modified, %d deleted\n",
			result.Summary.AddedCount, result.Summary.ModifiedCount, result.Summary.DeletedCount)
	}

	return result
}

// compareMerkleTrees uses Merkle tree comparison for efficient diff
func (d *Differ) compareMerkleTrees(baseline, current *snapshot.Snapshot, result *Result) {
	if d.config.Verbose {
		fmt.Printf("🌳 Using Merkle tree comparison...\n")
	}

	// Quick check: if root hashes are the same, no changes
	if baseline.MerkleRoot == current.MerkleRoot {
		if d.config.Verbose {
			fmt.Printf("✅ Merkle roots match - no changes detected\n")
		}
		return
	}

	if d.config.Verbose {
		fmt.Printf("🔍 Merkle roots differ - performing detailed comparison\n")
		fmt.Printf("   Baseline: %x\n", baseline.MerkleRoot[:8])
		fmt.Printf("   Current:  %x\n", current.MerkleRoot[:8])
	}

	// Since merkle roots differ, fall back to brute force comparison
	// In a full implementation, you could do more sophisticated tree comparison
	d.compareBruteForce(baseline, current, result)
}

// compareBruteForce performs traditional file-by-file comparison
func (d *Differ) compareBruteForce(baseline, current *snapshot.Snapshot, result *Result) {
	if d.config.Verbose {
		fmt.Printf("📊 Using brute force comparison...\n")
	}

	// Create a set of all unique paths
	allPaths := make(map[string]bool)
	for path := range baseline.Files {
		allPaths[path] = true
	}
	for path := range current.Files {
		allPaths[path] = true
	}

	processed := 0
	total := len(allPaths)

	for path := range allPaths {
		if d.ignorer.ShouldIgnore(path) {
			continue
		}

		baselineRecord, inBaseline := baseline.Files[path]
		currentRecord, inCurrent := current.Files[path]

		if !inBaseline && inCurrent {
			// File was added
			result.Added[path] = currentRecord
		} else if inBaseline && !inCurrent {
			// File was deleted
			result.Deleted[path] = baselineRecord
		} else if inBaseline && inCurrent {
			// File exists in both - check if modified
			if !d.filesEqual(baselineRecord, currentRecord) {
				changes := d.detectChanges(baselineRecord, currentRecord)
				result.Modified[path] = &ChangeDetail{
					OldRecord: baselineRecord,
					NewRecord: currentRecord,
					Changes:   changes,
				}
			}
		}

		processed++
		if d.config.Verbose && processed%10000 == 0 {
			fmt.Printf("📊 Processed %d/%d files (%.1f%%)\n",
				processed, total, float64(processed)/float64(total)*100)
		}
	}
}

// filesEqual checks if two file records are equal
func (d *Differ) filesEqual(a, b *snapshot.FileRecord) bool {
	if a.IsDir && b.IsDir {
		// For directories, compare metadata
		return a.Mode == b.Mode && a.ModTime.Equal(b.ModTime) && a.UID == b.UID && a.GID == b.GID
	}

	if a.IsDir != b.IsDir {
		return false
	}

	// For files, compare hash, size, and metadata
	return a.Hash == b.Hash &&
		a.Size == b.Size &&
		a.Mode == b.Mode &&
		a.UID == b.UID &&
		a.GID == b.GID
}

// detectChanges identifies what specifically changed about a file
func (d *Differ) detectChanges(old, new *snapshot.FileRecord) []string {
	var changes []string

	if old.Hash != new.Hash && old.Hash != "" && new.Hash != "" {
		changes = append(changes, "content")
	}

	if old.Size != new.Size {
		changes = append(changes, fmt.Sprintf("size (%d → %d)", old.Size, new.Size))
	}

	if old.Mode != new.Mode {
		changes = append(changes, fmt.Sprintf("permissions (%s → %s)", old.Mode, new.Mode))
	}

	if !old.ModTime.Equal(new.ModTime) {
		changes = append(changes, fmt.Sprintf("mtime (%s → %s)",
			old.ModTime.Format("2006-01-02 15:04:05"),
			new.ModTime.Format("2006-01-02 15:04:05")))
	}

	if old.UID != new.UID {
		changes = append(changes, fmt.Sprintf("uid (%d → %d)", old.UID, new.UID))
	}

	if old.GID != new.GID {
		changes = append(changes, fmt.Sprintf("gid (%d → %d)", old.GID, new.GID))
	}

	if len(changes) == 0 {
		changes = append(changes, "unknown")
	}

	return changes
}

// calculateSummary calculates summary statistics
func (d *Differ) calculateSummary(result *Result, duration time.Duration) Summary {
	summary := Summary{
		AddedCount:     len(result.Added),
		ModifiedCount:  len(result.Modified),
		DeletedCount:   len(result.Deleted),
		ComparisonTime: duration,
	}

	summary.TotalChanges = summary.AddedCount + summary.ModifiedCount + summary.DeletedCount

	// Calculate size changes
	for _, record := range result.Added {
		summary.AddedSize += record.Size
	}

	for _, record := range result.Deleted {
		summary.DeletedSize += record.Size
	}

	summary.SizeDiff = summary.AddedSize - summary.DeletedSize

	return summary
}

// ShouldIgnore checks if a path should be ignored during diff
func (i *PathIgnorer) ShouldIgnore(path string) bool {
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

// GetChangesByType returns changes grouped by type
func (r *Result) GetChangesByType() map[ChangeType][]string {
	changes := make(map[ChangeType][]string)

	for path := range r.Added {
		changes[ChangeAdded] = append(changes[ChangeAdded], path)
	}

	for path := range r.Modified {
		changes[ChangeModified] = append(changes[ChangeModified], path)
	}

	for path := range r.Deleted {
		changes[ChangeDeleted] = append(changes[ChangeDeleted], path)
	}

	// Sort for consistent output
	for _, paths := range changes {
		sort.Strings(paths)
	}

	return changes
}

// GetCriticalChanges returns potentially security-relevant changes
func (r *Result) GetCriticalChanges() []CriticalChange {
	var critical []CriticalChange

	criticalPaths := []string{
		"/etc/passwd", "/etc/shadow", "/etc/sudoers", "/etc/hosts",
		"/bin/", "/sbin/", "/usr/bin/", "/usr/sbin/",
		"/boot/", "/etc/systemd/", "/etc/cron", "/etc/ssh/",
		"/.ssh/", "/root/", "/home/", "/etc/security/",
		"/lib/systemd/", "/usr/lib/systemd/", "/etc/init.d/",
	}

	checkCritical := func(path string, changeType ChangeType, record *snapshot.FileRecord) {
		for _, critPath := range criticalPaths {
			if strings.Contains(path, critPath) {
				critical = append(critical, CriticalChange{
					Path:     path,
					Type:     changeType,
					Record:   record,
					Severity: calculateSeverity(path, changeType),
					Reason:   explainCriticality(path, changeType),
				})
				break
			}
		}
	}

	for path, record := range r.Added {
		checkCritical(path, ChangeAdded, record)
	}

	for path, change := range r.Modified {
		checkCritical(path, ChangeModified, change.NewRecord)
	}

	for path, record := range r.Deleted {
		checkCritical(path, ChangeDeleted, record)
	}

	// Sort by severity
	sort.Slice(critical, func(i, j int) bool {
		return critical[i].Severity > critical[j].Severity
	})

	return critical
}

// CriticalChange represents a security-relevant change
type CriticalChange struct {
	Path     string               `json:"path"`
	Type     ChangeType           `json:"type"`
	Record   *snapshot.FileRecord `json:"record"`
	Severity int                  `json:"severity"` // 1-10 scale
	Reason   string               `json:"reason"`
}

// calculateSeverity assigns a severity score to a critical change
func calculateSeverity(path string, changeType ChangeType) int {
	// High severity paths
	highSeverity := []string{
		"/etc/passwd", "/etc/shadow", "/etc/sudoers",
		"/bin/", "/sbin/", "/usr/bin/", "/usr/sbin/",
		"/root/", "/.ssh/",
	}

	for _, highPath := range highSeverity {
		if strings.Contains(path, highPath) {
			switch changeType {
			case ChangeAdded, ChangeModified:
				return 9
			case ChangeDeleted:
				return 8
			}
		}
	}

	// Medium severity
	mediumSeverity := []string{
		"/etc/", "/boot/", "/home/",
	}

	for _, medPath := range mediumSeverity {
		if strings.Contains(path, medPath) {
			switch changeType {
			case ChangeAdded, ChangeModified:
				return 6
			case ChangeDeleted:
				return 5
			}
		}
	}

	return 3 // Default severity
}

// explainCriticality explains why a change is considered critical
func explainCriticality(path string, changeType ChangeType) string {
	if strings.Contains(path, "/etc/passwd") {
		return "User account database modified"
	}
	if strings.Contains(path, "/etc/shadow") {
		return "Password hash database modified"
	}
	if strings.Contains(path, "/etc/sudoers") {
		return "Sudo privileges configuration modified"
	}
	if strings.Contains(path, "/bin/") || strings.Contains(path, "/sbin/") {
		return "System binary modified"
	}
	if strings.Contains(path, "/.ssh/") {
		return "SSH configuration or keys modified"
	}
	if strings.Contains(path, "/root/") {
		return "Root user directory modified"
	}
	if strings.Contains(path, "/etc/systemd/") {
		return "Systemd service configuration modified"
	}
	if strings.Contains(path, "/etc/cron") {
		return "Scheduled task configuration modified"
	}

	return fmt.Sprintf("Critical system path %s", string(changeType))
}

// FilterChanges filters the diff result based on criteria
func (r *Result) FilterChanges(filter func(path string, changeType ChangeType) bool) *Result {
	filtered := &Result{
		Baseline:  r.Baseline,
		Current:   r.Current,
		Added:     make(map[string]*snapshot.FileRecord),
		Modified:  make(map[string]*ChangeDetail),
		Deleted:   make(map[string]*snapshot.FileRecord),
		Generated: r.Generated,
	}

	for path, record := range r.Added {
		if filter(path, ChangeAdded) {
			filtered.Added[path] = record
		}
	}

	for path, change := range r.Modified {
		if filter(path, ChangeModified) {
			filtered.Modified[path] = change
		}
	}

	for path, record := range r.Deleted {
		if filter(path, ChangeDeleted) {
			filtered.Deleted[path] = record
		}
	}

	// Recalculate summary
	filtered.Summary = Summary{
		AddedCount:    len(filtered.Added),
		ModifiedCount: len(filtered.Modified),
		DeletedCount:  len(filtered.Deleted),
		TotalChanges:  len(filtered.Added) + len(filtered.Modified) + len(filtered.Deleted),
	}

	return filtered
}

// ExportCSV exports the diff results to CSV format
func (r *Result) ExportCSV() [][]string {
	var rows [][]string

	// Header
	rows = append(rows, []string{
		"Path", "Type", "Size", "Mode", "ModTime", "Hash", "Changes",
	})

	// Added files
	for path, record := range r.Added {
		rows = append(rows, []string{
			path, "added", fmt.Sprintf("%d", record.Size),
			record.Mode.String(), record.ModTime.Format("2006-01-02 15:04:05"),
			record.Hash, "",
		})
	}

	// Modified files
	for path, change := range r.Modified {
		rows = append(rows, []string{
			path, "modified", fmt.Sprintf("%d", change.NewRecord.Size),
			change.NewRecord.Mode.String(), change.NewRecord.ModTime.Format("2006-01-02 15:04:05"),
			change.NewRecord.Hash, strings.Join(change.Changes, "; "),
		})
	}

	// Deleted files
	for path, record := range r.Deleted {
		rows = append(rows, []string{
			path, "deleted", fmt.Sprintf("%d", record.Size),
			record.Mode.String(), record.ModTime.Format("2006-01-02 15:04:05"),
			record.Hash, "",
		})
	}

	return rows
}
