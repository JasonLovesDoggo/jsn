Various experimental things. This is my experimental monorepo where programs will be played with before they are moved out into other repos. This repo has a target audience of one: me.

## Projects

### Command-line Tools (`cmd/`)

- **fsdiff** - High-performance filesystem diff tool using Merkle trees and parallel processing, designed for cybersecurity competitions and system administration
- **httpdebug** - HTTP header debugging server that displays incoming request headers  
- **pkg.jsn.cam** - Vanity import server for Go packages hosted at https://pkg.jsn.cam
- **portkill** - Simple utility to kill processes by port number with support for force killing and listing
- **revproxyd** - Basic reverse proxy daemon for forwarding HTTP requests
- **serve** - Static file server with optional verbose logging
- **typer** - Go code generation tool 

### Libraries (`pkg/`)

- **flagr** - Minimal Go package extending the standard `flag` package to support both long and short form flags (`--verbose` and `-v`)
- **flagenv** - Library for populating command-line flags from environment variables

### Web Components (`jass/`)

- **jass** - Vendored CSS framework package that provides embedded stylesheets for web applications

### Internal Utilities (`internal/`)

- Build info, caching, licensing, logging, middleware, and other shared utilities used across projects
