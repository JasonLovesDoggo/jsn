// Package jass vendors a copy of Jass and makes it available at /.jass/jass.css
//
// This is intended to be used as a vendored package in other projects.
package jass

import (
	"embed"
	"github.com/jasonlovesdoggo/jsn/internal"
	"net/http"
	"within.website/x"

	"github.com/a-h/templ"
)

//go:generate go tool templ generate

var (
	//go:embed jass.css static
	Static embed.FS

	URL = "/.jsn.cam/jsn/jass/jass.css"
)

func init() {
	Mount(http.DefaultServeMux)

	URL = URL + "?cachebuster=" + x.Version
}

func Mount(mux *http.ServeMux) {
	mux.Handle("/.jsn.cam/jsn/jass", internal.UnchangingCache(http.StripPrefix("/.jsn.cam/jsn/jass", http.FileServerFS(Static))))
}

func NotFound(w http.ResponseWriter, r *http.Request) {
	templ.Handler(
		Simple("Not found: "+r.URL.Path, notfound(r.URL.Path)),
		templ.WithStatus(http.StatusNotFound),
	)
}
