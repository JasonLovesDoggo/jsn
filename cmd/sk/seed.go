package main

import (
	"os"
	"path/filepath"
	"strings"

	"pkg.jsn.cam/jsn/pkg/skit/defaults"
)

func seedScriptsIfEmpty(dest string) {
	entries, err := os.ReadDir(dest)
	if err != nil || len(entries) > 0 {
		return
	}
	if !defaults.HasDefaults() {
		return
	}
	if err := defaults.CopyScripts(dest); err != nil {
		// seeding is best-effort; log to stderr but do not exit
		_, _ = os.Stderr.WriteString("skit: failed to copy default scripts: " + err.Error() + "\n")
	}
	// ensure scripts are executable if embedded perms were lost
	filepath.WalkDir(dest, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".sh") {
			_ = os.Chmod(path, 0o755)
		}
		return nil
	})
}
