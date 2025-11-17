package defaults

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed scripts/**
var scriptsFS embed.FS

// CopyScripts copies the embedded default scripts into dest if dest is empty.
func CopyScripts(dest string) error {
	return fs.WalkDir(scriptsFS, "scripts", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(path, "scripts")
		if rel == "" {
			return nil
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		mode := info.Mode() & os.ModePerm
		if mode == 0 {
			mode = 0o644
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		src, err := scriptsFS.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		dst, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
		if err != nil {
			return err
		}
		if _, err := io.Copy(dst, src); err != nil {
			closeErr := dst.Close()
			if closeErr != nil {
				return fmt.Errorf("copy error: %v; close error: %v", err, closeErr)
			}
			return err
		}
		return dst.Close()
	})
}

// HasDefaults reports whether any embedded scripts exist.
func HasDefaults() bool {
	entries, err := fs.ReadDir(scriptsFS, "scripts")
	return err == nil && len(entries) > 0
}

// List returns the relative paths of embedded scripts for documentation/testing.
func List() ([]string, error) {
	var out []string
	err := fs.WalkDir(scriptsFS, "scripts", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || strings.HasSuffix(path, "/") {
			return nil
		}
		rel := strings.TrimPrefix(path, "scripts/")
		if rel != "" {
			out = append(out, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list defaults: %w", err)
	}
	return out, nil
}
