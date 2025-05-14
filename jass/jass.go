// Package jass vendors a copy of Jass and makes it available at /.jass/jass.css
//
// This is intended to be used as a vendored package in other projects.
package jass

import (
	"embed"
	"net/http"

	"github.com/a-h/templ"
	"pkg.jsn.cam/jsn"
	"pkg.jsn.cam/jsn/internal"
)

//go:generate go tool templ generate

var (
	//go:embed jass.min.css static
	Static embed.FS

	URL    = "/.jsn.cam/jass/jass.min.css"
	prefix = "/.jsn.cam/jass/"
)

func init() {
	Mount(http.DefaultServeMux)
	URL = URL + "?cachebuster=" + jsn.Version
}

func Mount(mux *http.ServeMux) {
	mux.Handle(prefix, http.StripPrefix(prefix, internal.UnchangingCache(http.FileServerFS(Static))))
}

func NotFound(w http.ResponseWriter, r *http.Request) {
	templ.Handler(
		Simple("Not found: "+r.URL.Path, notfound(r.URL.Path)),
		templ.WithStatus(http.StatusNotFound),
	)
}
