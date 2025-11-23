package main

import (
	"fmt"
	"os"
	"path/filepath"
)

type paths struct {
	Home       string
	ScriptsDir string
	StateDir   string
}

func resolvePaths() (paths, error) {
	home := os.Getenv("SKIT_HOME")
	if home == "" {
		if dir, err := os.UserHomeDir(); err == nil && dir != "" {
			home = filepath.Join(dir, ".skit")
		} else {
			home = filepath.Join(os.TempDir(), "skit")
		}
	}
	p := paths{
		Home:       home,
		ScriptsDir: filepath.Join(home, "scripts"),
		StateDir:   filepath.Join(home, "state"),
	}
	if err := os.MkdirAll(p.ScriptsDir, 0o755); err != nil {
		return paths{}, fmt.Errorf("ensure scripts dir: %w", err)
	}
	if err := os.MkdirAll(p.StateDir, 0o755); err != nil {
		return paths{}, fmt.Errorf("ensure state dir: %w", err)
	}
	return p, nil
}
