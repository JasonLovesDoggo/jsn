// Command pkg.jsn.cam is the vanity import server for https://pkg.jsn.cam
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/a-h/templ"
	"github.com/jasonlovesdoggo/jsn/internal"
	"github.com/jasonlovesdoggo/jsn/jass"
)

//go:generate go tool templ generate

var (
	domain      = flag.String("domain", "pkg.jsn.cam", "domain this is run on")
	port        = flag.String("port", "2143", "HTTP port to listen on")
	metricsPort = flag.String("metrics-port", "9091", "Prometheus metrics HTTP port")
	tomlConfig  = flag.String("config", "./config.toml", "TOML config file")
)

func main() {
	internal.HandleStartup()
	flag.Parse()

	lg := slog.Default().With("domain", *domain, "configPath", *tomlConfig)

	// Resolve path relative to executable

	configPath := *tomlConfig
	lg.Debug("loading config", "path", configPath)

	// Load config and repositories from TOML file
	config, err := LoadConfig(configPath, lg)
	if err != nil {
		lg.Error("can't decode config at either path",
			"path", configPath,
			"err", err)
		os.Exit(1)
	}

	// Build the list of repositories from the config
	repos := BuildRepos(config, lg)

	// Debug logging for repos
	lg.Debug("loaded repos", "count", len(repos))
	for i, repo := range repos {
		lg.Debug("loaded repo", "index", i, "repo", repo)
	}

	mux := http.NewServeMux()

	// Start metrics server on separate port
	RegisterMetricsHandler(*metricsPort, lg)

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

	// Wrap the mux with the metrics middleware
	handler := MetricsMiddleware(mux)

	err = http.ListenAndServe(":"+*port, handler)
	if err != nil {
		lg.Error("can't start server", "err", err)
		os.Exit(1)
	}
}
