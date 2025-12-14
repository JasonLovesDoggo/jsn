package live

import (
	"regexp"
	"sort"
	"time"
)

// BulkPattern defines a pattern for bulk change detection
type BulkPattern struct {
	Name    string
	Pattern *regexp.Regexp
	MinHits int
}

// BulkGroup represents a group of related changes
type BulkGroup struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	StartTime time.Time `json:"start"`
	EndTime   time.Time `json:"end"`
	Count     int       `json:"count"`
	Paths     []string  `json:"paths,omitempty"` // First few paths as sample
}

// Default bulk patterns for common operations
var defaultBulkPatterns = []BulkPattern{
	{Name: "apt_install", Pattern: regexp.MustCompile(`^/(usr|var/lib/dpkg|var/cache/apt)/`), MinHits: 20},
	{Name: "npm_install", Pattern: regexp.MustCompile(`node_modules/`), MinHits: 50},
	{Name: "pip_install", Pattern: regexp.MustCompile(`(site-packages|dist-packages)/`), MinHits: 20},
	{Name: "go_mod", Pattern: regexp.MustCompile(`/go/pkg/mod/`), MinHits: 20},
	{Name: "cargo_build", Pattern: regexp.MustCompile(`/target/(debug|release)/`), MinHits: 30},
	{Name: "git_checkout", Pattern: regexp.MustCompile(`\.git/objects/`), MinHits: 10},
}

// BulkDetector identifies bulk operations in change streams
type BulkDetector struct {
	patterns   []BulkPattern
	windowSize time.Duration
	genericMin int // Minimum changes for generic bulk detection
	nextBulkID int
}

// NewBulkDetector creates a new bulk detector with default settings
func NewBulkDetector() *BulkDetector {
	return &BulkDetector{
		patterns:   defaultBulkPatterns,
		windowSize: 5 * time.Second,
		genericMin: 30, // Any 30+ changes in 5s = bulk
		nextBulkID: 1,
	}
}

// DetectBulkGroups analyzes changes and assigns bulk IDs
// Returns the detected bulk groups and modifies change.BulkID in place
func (d *BulkDetector) DetectBulkGroups(changes []*Change) []BulkGroup {
	if len(changes) == 0 {
		return nil
	}

	// Sort by timestamp
	sorted := make([]*Change, len(changes))
	copy(sorted, changes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	var groups []BulkGroup

	// Sliding window detection
	windowStart := 0
	for windowEnd := 0; windowEnd < len(sorted); windowEnd++ {
		// Move window start to keep within time window
		for windowStart < windowEnd &&
			sorted[windowEnd].Timestamp.Sub(sorted[windowStart].Timestamp) > d.windowSize {
			windowStart++
		}

		windowChanges := sorted[windowStart : windowEnd+1]
		if len(windowChanges) < d.genericMin {
			continue
		}

		// Check pattern-based detection
		group := d.detectPatternBulk(windowChanges)
		if group != nil {
			group.ID = d.nextBulkID
			d.nextBulkID++

			// Mark changes as part of this bulk
			for _, c := range windowChanges {
				if d.matchesPattern(c.Path, group.Name) {
					c.BulkID = group.ID
				}
			}

			groups = append(groups, *group)
			// Skip past this window
			windowStart = windowEnd + 1
			continue
		}

		// Generic bulk detection (any large group of changes)
		if len(windowChanges) >= d.genericMin {
			group := &BulkGroup{
				ID:        d.nextBulkID,
				Name:      "bulk_operation",
				StartTime: windowChanges[0].Timestamp,
				EndTime:   windowChanges[len(windowChanges)-1].Timestamp,
				Count:     len(windowChanges),
				Paths:     extractSamplePaths(windowChanges, 5),
			}
			d.nextBulkID++

			for _, c := range windowChanges {
				c.BulkID = group.ID
			}

			groups = append(groups, *group)
			windowStart = windowEnd + 1
		}
	}

	return groups
}

// detectPatternBulk checks if changes match a known bulk pattern
func (d *BulkDetector) detectPatternBulk(changes []*Change) *BulkGroup {
	for _, pattern := range d.patterns {
		matchCount := 0
		var matchingChanges []*Change

		for _, c := range changes {
			if pattern.Pattern.MatchString(c.Path) {
				matchCount++
				matchingChanges = append(matchingChanges, c)
			}
		}

		if matchCount >= pattern.MinHits {
			return &BulkGroup{
				Name:      pattern.Name,
				StartTime: matchingChanges[0].Timestamp,
				EndTime:   matchingChanges[len(matchingChanges)-1].Timestamp,
				Count:     matchCount,
				Paths:     extractSamplePaths(matchingChanges, 5),
			}
		}
	}
	return nil
}

// matchesPattern checks if a path matches a named pattern
func (d *BulkDetector) matchesPattern(path, patternName string) bool {
	for _, p := range d.patterns {
		if p.Name == patternName && p.Pattern.MatchString(path) {
			return true
		}
	}
	// For generic bulk_operation, always match
	return patternName == "bulk_operation"
}

// extractSamplePaths gets a sample of paths from changes
func extractSamplePaths(changes []*Change, limit int) []string {
	paths := make([]string, 0, limit)
	for i, c := range changes {
		if i >= limit {
			break
		}
		paths = append(paths, c.Path)
	}
	return paths
}

// IsBulk returns true if the change is part of a bulk operation
func IsBulk(c *Change) bool {
	return c.BulkID > 0
}
