package internal

import (
	"github.com/jasonlovesdoggo/jsn"
	"net/http"
)

func UnchangingCache(h http.Handler) http.Handler {
	//goland:noinspection GoBoolExpressions
	if jsn.Version == "devel" {
		return h
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		h.ServeHTTP(w, r)
	})
}
