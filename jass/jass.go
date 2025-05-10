// Package jass vendors a copy of Jass and makes it available at /.jass/jass.css
//
// This is intended to be used as a vendored package in other projects.
package jass

import (
	"embed"
	"github.com/a-h/templ"
	"github.com/jasonlovesdoggo/jsn"
	"github.com/jasonlovesdoggo/jsn/internal"
	"net/http"
)

//go:generate go tool templ generate

var (
	//go:embed jass.css static
	Static embed.FS

	URL    = "/.jsn.cam/jass/jass.css"
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
