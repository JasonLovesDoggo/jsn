// Command pkg.jsn.cam is the vanity import server for https://pkg.jsn.cam
package main

import (
	"flag"
	"fmt"
	"github.com/a-h/templ"
	"github.com/jasonlovesdoggo/jsn/internal"
	"github.com/jasonlovesdoggo/jsn/internal/vanity"
	"github.com/jasonlovesdoggo/jsn/jass"
	"go.jetpack.io/tyson"
	"log/slog"
	"net/http"
	"os"
)

//go:generate go tool templ generate ./...

var (
	domain      = flag.String("domain", "pkg.jsn.cam", "domain this is run on")
	port        = flag.String("port", "2134", "HTTP port to listen on")
	tysonConfig = flag.String("tyson-config", "./config.ts", "TySON config file")
)

type Repo struct {
	Kind        string `json:"kind"`
	Domain      string `json:"domain"`
	User        string `json:"user"`
	Repo        string `json:"repo"`
	Description string `json:"description"`
}

func (r Repo) URL() string {
	return fmt.Sprintf("https://%s/%s/%s", r.Domain, r.User, r.Repo)
}

func (r Repo) GodocURL() string {
	return fmt.Sprintf("https://pkg.go.dev/%s/%s", r.Domain, r.Repo)
}

func (r Repo) GodocBadge() string {
	return fmt.Sprintf("https://pkg.go.dev/badge/%s/%s.svg", r.Domain, r.Repo)
}

func (r Repo) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("kind", r.Kind),
		slog.String("domain", r.Domain),
		slog.String("user", r.User),
		slog.String("repo", r.Repo),
	)
}

func (r Repo) RegisterHandlers(mux *http.ServeMux, lg *slog.Logger) {
	switch r.Kind {
	case "gitea":
		mux.Handle("/"+r.Repo, vanity.GogsHandler(*domain+"/"+r.Repo, r.Domain, r.User, r.Repo, "https"))
		mux.Handle("/"+r.Repo+"/", vanity.GogsHandler(*domain+"/"+r.Repo, r.Domain, r.User, r.Repo, "https"))
	case "github":
		mux.Handle("/"+r.Repo, vanity.GitHubHandler(*domain+"/"+r.Repo, r.User, r.Repo, "https"))
		mux.Handle("/"+r.Repo+"/", vanity.GitHubHandler(*domain+"/"+r.Repo, r.User, r.Repo, "https"))
	}
	lg.Debug("registered repo handler", "repo", r)
}

//go:generate go tool templ generate

func main() {
	internal.HandleStartup()

	lg := slog.Default().With("domain", *domain, "configPath", *tysonConfig)

	var repos []Repo
	if err := tyson.Unmarshal(*tysonConfig, &repos); err != nil {
		lg.Error("can't unmarshal config", "err", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()

	for _, repo := range repos {
		repo.RegisterHandlers(mux, lg)
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
	http.ListenAndServe(":"+*port, mux)
}
