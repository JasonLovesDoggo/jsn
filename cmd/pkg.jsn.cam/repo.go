package main

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jasonlovesdoggo/jsn/internal/vanity"
)

// Repo represents a repository with its metadata
type Repo struct {
	Kind        string
	Domain      string
	User        string
	Repo        string
	Description string
}

// URL returns the full URL to the repository
func (r Repo) URL() string {
	return fmt.Sprintf("https://%s/%s/%s", r.Domain, r.User, r.Repo)
}

// GodocURL returns the URL to view the package documentation on pkg.go.dev
func (r Repo) GodocURL() string {
	return fmt.Sprintf("https://pkg.go.dev/%s/%s", *domain, r.Repo)
}

// GodocBadge returns the URL to the pkg.go.dev badge for this repository
func (r Repo) GodocBadge() string {
	return fmt.Sprintf("https://pkg.go.dev/badge/%s/%s/%s.svg", r.Domain, r.User, r.Repo)
}

// LogValue implements slog.LogValuer to provide structured logging
func (r Repo) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("kind", r.Kind),
		slog.String("domain", r.Domain),
		slog.String("user", r.User),
		slog.String("repo", r.Repo),
	)
}

// RegisterHandlers registers HTTP handlers for this repository
func (r Repo) RegisterHandlers(mux *http.ServeMux, domain string, lg *slog.Logger) {
	switch r.Kind {
	case "gitea":
		mux.Handle("/"+r.Repo, vanity.GogsHandler(domain+"/"+r.Repo, r.Domain, r.User, r.Repo, "https"))
		mux.Handle("/"+r.Repo+"/", vanity.GogsHandler(domain+"/"+r.Repo, r.Domain, r.User, r.Repo, "https"))
	case "github":
		mux.Handle("/"+r.Repo, vanity.GitHubHandler(domain+"/"+r.Repo, r.User, r.Repo, "https"))
		mux.Handle("/"+r.Repo+"/", vanity.GitHubHandler(domain+"/"+r.Repo, r.User, r.Repo, "https"))
	case "gitlab":
		mux.Handle("/"+r.Repo, vanity.GitHubHandler(domain+"/"+r.Repo, r.User, r.Repo, "https"))
		mux.Handle("/"+r.Repo+"/", vanity.GitHubHandler(domain+"/"+r.Repo, r.User, r.Repo, "https"))
	}
	lg.Debug("registered repo handler", "repo", r)
}
