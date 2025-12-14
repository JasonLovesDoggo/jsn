package scanner

import (
	"path/filepath"
	"strings"
)

type PathIgnorer struct {
	patterns map[string]bool
	prefixes []string
	suffixes []string
	contains []string
}

func newPathIgnorer(userPatterns []string) *PathIgnorer {
	defaultPatterns := []string{
		// Linux virtual/temp filesystems
		"/proc", "/sys", "/dev", "/tmp", "/var/tmp", "/run", "/var/run",
		"/var/log", "/var/cache", "/var/lib/dhcp",
		"/home/*/.cache", "/home/*/.local/share/Trash",
		"/home/*/.mozilla/firefox/*/Cache",
		"/home/*/.config/google-chrome/*/Cache",
		"/var/lib/docker/overlay2", "/var/lib/containerd",

		// macOS - exclude mounted volumes (NAS, external drives) by default
		"/Volumes",
		"/System/Volumes/Data", // APFS firmlinks
		"/.Spotlight-V100", "/.fseventsd", "/.Trashes",
		"/Library/Caches", "/Users/*/Library/Caches",
		"/private/var/folders", // macOS temp

		// macOS - directories that trigger permission prompts
		"/Library/Application Support/com.apple.TCC", // Privacy database
		"/private/var/db/dslocal",                    // Directory services
		"/private/var/db/uuidtext",                   // System diagnostics
		"/Users/*/Library/Application Support/MobileSync",
		"/Users/*/Library/Containers",
		"/Users/*/Library/Group Containers",
		"/Users/*/Library/Messages",
		"/Users/*/Library/Mail",
		"/Users/*/Library/Safari",
		"/Users/*/Library/Calendars",
		"/Users/*/Library/Reminders",
		"/Users/*/Library/HomeKit",
		"/Users/*/Library/IdentityServices",
		"/Users/*/Library/Metadata/CoreSpotlight",
		"/Users/*/Library/PersonalizationPortrait",
		"/Users/*/Library/Suggestions",
		"/Users/*/Pictures/Photos Library.photoslibrary",
		"/Users/*/Library/Photos",

		// Common dev/build artifacts
		"/.cache", "node_modules",
		".git", ".svn", ".hg", "__pycache__", ".pytest_cache",
		"*.pyc", "*.pyo", "*.swp", "*.bak", "*~", "*.log", "*.tmp",
	}

	ignorer := &PathIgnorer{
		patterns: make(map[string]bool),
		prefixes: make([]string, 0, 32),
		suffixes: make([]string, 0, 32),
		contains: make([]string, 0, 32),
	}

	// Pre-process patterns for faster matching
	allPatterns := append(defaultPatterns, userPatterns...)
	for _, pattern := range allPatterns {
		if strings.HasPrefix(pattern, "*/") {
			ignorer.suffixes = append(ignorer.suffixes, pattern[1:])
		} else if strings.HasSuffix(pattern, "/*") {
			ignorer.prefixes = append(ignorer.prefixes, pattern[:len(pattern)-1])
		} else if strings.Contains(pattern, "*") {
			// Keep as pattern for glob matching
			ignorer.patterns[pattern] = true
		} else if strings.HasPrefix(pattern, "/") {
			ignorer.prefixes = append(ignorer.prefixes, pattern)
		} else {
			ignorer.contains = append(ignorer.contains, pattern)
		}
	}

	return ignorer
}

func (i *PathIgnorer) ShouldIgnore(path string) bool {
	// Fast exact match
	if i.patterns[path] {
		return true
	}

	// Fast prefix match
	for _, prefix := range i.prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	// Fast suffix match
	for _, suffix := range i.suffixes {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}

	// Fast contains match
	for _, contain := range i.contains {
		if strings.Contains(path, contain) {
			return true
		}
	}

	// Slower glob matching
	base := filepath.Base(path)
	for pattern := range i.patterns {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	}

	return false
}
