package internal

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"

	"pkg.jsn.cam/jsn"
)

func init() {
	http.HandleFunc("/.jsn/debug/buildinfo", func(w http.ResponseWriter, r *http.Request) {
		bi, ok := debug.ReadBuildInfo()
		if !ok {
			slog.Error("can't read build info")
			http.Error(w, "no build info available", http.StatusInternalServerError)
			return
		}

		if err := json.NewEncoder(w).Encode(struct {
			BuildInfo *debug.BuildInfo `json:"build_info"`
			Version   string           `json:"version"`
		}{bi, jsn.Version}); err != nil {
			slog.Error("can't encode build info", "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
