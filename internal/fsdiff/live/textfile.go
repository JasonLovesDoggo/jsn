package live

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Text file extensions (fast path detection)
var textExts = map[string]struct{}{
	".txt": {}, ".md": {}, ".json": {}, ".yaml": {}, ".yml": {}, ".toml": {},
	".go": {}, ".py": {}, ".js": {}, ".ts": {}, ".html": {}, ".css": {},
	".sh": {}, ".conf": {}, ".cfg": {}, ".ini": {}, ".env": {}, ".log": {},
}

// Config files without extensions
var configFiles = map[string]struct{}{
	"Makefile": {}, "Dockerfile": {}, "Vagrantfile": {},
	".bashrc": {}, ".zshrc": {}, ".profile": {},
	"passwd": {}, "shadow": {}, "group": {}, "hosts": {},
	"fstab": {}, "crontab": {}, "sudoers": {},
}

// isTextFile returns true if the file is likely a text file
func isTextFile(path string) bool {
	// Fast path: known text extensions
	if _, ok := textExts[strings.ToLower(filepath.Ext(path))]; ok {
		return true
	}

	// Known extensionless config files
	if _, ok := configFiles[filepath.Base(path)]; ok {
		return true
	}

	// Config-centric heuristic for /etc
	if strings.HasPrefix(path, "/etc/") {
		return isTextContent(path)
	}

	return false
}

// isTextContent reads a sample and checks if it's valid text
func isTextContent(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 1024)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return false
	}
	s := buf[:n]

	// Hard binary rejects (magic bytes)
	if hasBinaryMagic(s) {
		return false
	}

	// UTF-16 BOM support
	if bytes.HasPrefix(s, []byte{0xFF, 0xFE}) || bytes.HasPrefix(s, []byte{0xFE, 0xFF}) {
		return true
	}

	// UTF-8 validation
	if !utf8.Valid(s) {
		return false
	}

	printable := 0
	for _, b := range s {
		if b == '\n' || b == '\r' || b == '\t' || (b >= 32 && b <= 126) {
			printable++
		}
	}

	return float64(printable)/float64(len(s)) > 0.7
}

// hasBinaryMagic checks for known binary file magic bytes
func hasBinaryMagic(b []byte) bool {
	magics := [][]byte{
		{0x7f, 'E', 'L', 'F'},  // ELF
		{0x1f, 0x8b},           // gzip
		{'P', 'K', 0x03, 0x04}, // zip
		{0x89, 'P', 'N', 'G'},  // png
		{'%', 'P', 'D', 'F'},   // pdf
	}
	for _, m := range magics {
		if len(b) >= len(m) && bytes.Equal(b[:len(m)], m) {
			return true
		}
	}
	return false
}
