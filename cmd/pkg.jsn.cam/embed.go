package main

import (
	"embed"
	"io/fs"
	"os"
	"strings"
)

//go:embed config.toml
var defaults embed.FS

func Open(path string) (fs.File, error) {
	if strings.HasPrefix(path, "(data)/") {
		fname := strings.TrimPrefix(path, "(data)/")
		fin, err := defaults.Open(fname)
		return fin, err
	}
	return os.Open(path)
}
