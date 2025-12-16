package live

import (
	"os"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	"pkg.jsn.cam/jsn/internal/fsdiff/snapshot"
)

// captureContent captures file content for text files.
// If change is nil, just stores content without updating a change record (for baseline capture).
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
		if change != nil {
			change.ContentKey = record.Hash
		}
		return
	}

	// Read and save content
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}

	if err := s.store.SaveContent(record.Hash, content); err == nil {
		if change != nil {
			change.ContentKey = record.Hash
		}
	}
}

// shouldComputeDiff returns true if the path is in a watched diff directory
func (s *Session) shouldComputeDiff(path string) bool {
	if len(s.config.DiffDirs) == 0 {
		return false
	}
	// Ensure path is under RootPath
	if !strings.HasPrefix(path, s.config.RootPath) {
		return false
	}
	for _, dir := range s.config.DiffDirs {
		// DiffDir must also be under RootPath
		if !strings.HasPrefix(dir, s.config.RootPath) {
			continue
		}
		if strings.HasPrefix(path, dir) {
			return true
		}
	}
	return false
}

// computeDiff generates a unified diff between old content and current file
func (s *Session) computeDiff(path, oldHash string) string {
	oldContent, err := s.store.LoadContent(oldHash)
	if err != nil {
		if s.config.Verbose {
			println("  computeDiff: failed to load old content for hash", oldHash[:8], ":", err.Error())
		}
		return ""
	}

	newContent, err := os.ReadFile(path)
	if err != nil {
		if s.config.Verbose {
			println("  computeDiff: failed to read current file", path, ":", err.Error())
		}
		return ""
	}

	// Split into lines - difflib expects lines WITH trailing \n
	oldText := string(oldContent)
	newText := string(newContent)

	// Ensure content ends with newline for consistent splitting
	if !strings.HasSuffix(oldText, "\n") {
		oldText += "\n"
	}
	if !strings.HasSuffix(newText, "\n") {
		newText += "\n"
	}

	oldLines := strings.SplitAfter(oldText, "\n")
	newLines := strings.SplitAfter(newText, "\n")

	// Remove empty last element if present
	if len(oldLines) > 0 && oldLines[len(oldLines)-1] == "" {
		oldLines = oldLines[:len(oldLines)-1]
	}
	if len(newLines) > 0 && newLines[len(newLines)-1] == "" {
		newLines = newLines[:len(newLines)-1]
	}

	return generateUnifiedDiff(path, oldLines, newLines)
}

// generateUnifiedDiff creates a proper unified diff using go-difflib
func generateUnifiedDiff(path string, oldLines, newLines []string) string {
	var buf strings.Builder

	diff := difflib.UnifiedDiff{
		A:        oldLines,
		B:        newLines,
		FromFile: "a/" + path,
		ToFile:   "b/" + path,
		Context:  3,
	}

	if err := difflib.WriteUnifiedDiff(&buf, diff); err != nil {
		return ""
	}

	return buf.String()
}
