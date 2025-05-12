// Command pkg.jsn.cam is the vanity import server for https://pkg.jsn.cam
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/a-h/templ"
	"github.com/jasonlovesdoggo/jsn/internal"
	"github.com/jasonlovesdoggo/jsn/jass"
)

//go:generate go tool templ generate

var (
	domain     = flag.String("domain", "pkg.jsn.cam", "domain this is run on")
	port       = flag.String("port", "2143", "HTTP port to listen on")
	tomlConfig = flag.String("config", "./config.toml", "TOML config file")
)

func main() {
	internal.HandleStartup()
	flag.Parse()

	lg := slog.Default().With("domain", *domain, "configPath", *tomlConfig)

	// Resolve path relative to executable
	execPath, err := os.Executable()
	if err != nil {
		lg.Error("can't get executable path", "err", err)
		os.Exit(1)
	}

	configPath := *tomlConfig
	if !filepath.IsAbs(configPath) {
		configPath = filepath.Join(filepath.Dir(execPath), configPath)
	}

	lg.Debug("loading config", "path", configPath)

	// Load config and repositories from TOML file
	config, err := LoadConfig(configPath, lg)
	if err != nil {
		// Try relative to working directory if that fails
		workingDir, _ := os.Getwd()
		altPath := filepath.Join(workingDir, *tomlConfig)

		lg.Debug("trying alternative config path", "path", altPath)

		config, err = LoadConfig(altPath, lg)
		if err != nil {
			lg.Error("can't decode config at either path",
				"primary_path", configPath,
				"alt_path", altPath,
				"err", err)
			os.Exit(1)
		}
	}

	// Build the list of repositories from the config
	repos := BuildRepos(config, lg)

	// Debug logging for repos
	lg.Debug("loaded repos", "count", len(repos))
	for i, repo := range repos {
		lg.Debug("loaded repo", "index", i, "repo", repo)
	}

	mux := http.NewServeMux()

	// Register handlers for each repository
	for _, repo := range repos {
		repo.RegisterHandlers(mux, *domain, lg)
	}

	jass.Mount(mux)

	mux.Handle("/{$}", templ.Handler(
		jass.Base(
			fmt.Sprintf("%s Go packages", *domain),
			nil,
			nil,
			Index(repos),
			footer(),
		),
	))

	mux.Handle("/", templ.Handler(
		jass.Simple("Not found", NotFound()),
		templ.WithStatus(http.StatusNotFound)),
	)

	mux.Handle("/.jsn.botinfo", templ.Handler(
		jass.Simple("jsn repo bots", BotInfo()),
	))

	lg.Info("listening", "port", *port)
	err = http.ListenAndServe(":"+*port, mux)
	if err != nil {
		lg.Error("can't start server", "err", err)
		os.Exit(1)
	}
}
