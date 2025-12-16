//go:generate bun run --cwd . build

package ui

import "embed"

//go:embed all:dist
var Dist embed.FS
