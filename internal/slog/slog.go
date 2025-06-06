// Package slog is my set of wrappers around package slog.
package slog

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
)

var (
	slogLevel = flag.String("slog-level", "INFO", "log level")

	// The current slog handler.
	Handler slog.Handler

	leveler *slog.LevelVar
)

func Init() {
	var programLevel slog.Level
	if err := (&programLevel).UnmarshalText([]byte(*slogLevel)); err != nil {
		fmt.Fprintf(os.Stderr, "invalid log level %s: %v, using info\n", *slogLevel, err)
		programLevel = slog.LevelInfo
	}

	leveler = &slog.LevelVar{}
	leveler.Set(programLevel)

	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: true,
		Level:     leveler,
	})
	slog.SetDefault(slog.New(h))

	Handler = h

	http.HandleFunc("/.jsn/debug/slog-level", func(w http.ResponseWriter, r *http.Request) {
		var level, old slog.Level
		old = leveler.Level()

		if r.Method == http.MethodPost {
			data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64))
			defer r.Body.Close()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			if err := (&level).UnmarshalText(data); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			leveler.Set(level)
			slog.Info("changed level", "from", old, "to", level)
			fmt.Fprintln(w, level)
		} else {
			fmt.Fprintln(w, old)
		}
	})
}
