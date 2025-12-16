package live

import (
	"fmt"
	"strings"
	"time"
)

// getIcon returns a single-character icon for a change type
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

// printChanges prints detected changes to stdout
func (s *Session) printChanges(changes []*Change) {
	for _, change := range changes {
		icon := getIcon(change.Type)
		fmt.Printf("%s %s %s\n", time.Now().Format("15:04:05"), icon, change.Path)
	}
}

// printSummary prints session summary to stdout
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
	type pathCount struct {
		path  string
		count int
	}
	var sorted []pathCount
	for path, count := range pathCounts {
		sorted = append(sorted, pathCount{path, count})
	}
	// Sort by count descending
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
