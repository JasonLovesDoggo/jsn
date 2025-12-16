//go:generate bun run --cwd . build

package ui

import "embed"

//go:embed dist/*
var Dist embed.FS
