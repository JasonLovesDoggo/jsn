Various experimental things. This is my experimental monorepo where programs will be played with before they are moved out into other repos. This repo has a target audience of one: me.

## Projects

### Command-line Tools (`cmd/`)

- **fsdiff** - High-performance filesystem diff tool using Merkle trees and parallel processing, like `git diff` but for your entire system
- **httpdebug** - HTTP header debugging server that displays incoming request headers, like `nc -l` but for HTTP debugging
- **pkg.jsn.cam** - Vanity import server for Go packages hosted at https://pkg.jsn.cam
- **portkill** - Simple utility to kill processes by port number, like `lsof -ti:PORT | xargs kill` but simpler
- **revproxyd** - Basic reverse proxy daemon for forwarding HTTP requests, like `nginx` proxy_pass but minimal
- **serve** - Static file server with optional verbose logging, like `python3 -m http.server` but faster and easier
- **backoff** - Retry commands with exponential backoff, like a simple `retry` wrapper for flaky commands
- **cidrpack** - A simple CLI to merge and minimize overlapping CIDR blocks
- **typein** - Types stdin or argument text like a human (uses shared humantype lib with typer)
- **sk** - Scriptkit: a fuzzy CLI to stash and run personal scripts under `~/.skit/scripts` (git-sync optional).
- **jason** - (TOY) JSON lexer CLI for tokenizing JSON files

### Assorted Tools (`cmd/assorted/`)

- **openrouter/tpsviz** - TUI for analyzing OpenRouter activity export logs and computing TPS metrics


### Libraries (`pkg/`)

- **flagr** - Minimal Go package extending the standard `flag` package to support both long and short form flags (`--verbose` and `-v`)
- **flagenv** - Library for populating command-line flags from environment variables

### Web Components (`jass/`)

- **jass** - Vendored CSS framework package that provides embedded stylesheets for web applications

### Internal Utilities (`internal/`)

- Build info, caching, licensing, logging, middleware, and other shared utilities used across projects
