package diff

import (
	"fmt"
	"sort"
	"strings"

	"pkg.jsn.cam/jsn/internal/fsdiff/snapshot"
)

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
