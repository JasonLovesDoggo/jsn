package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"pkg.jsn.cam/jsn/internal"
	"pkg.jsn.cam/jsn/internal/fsdiff/live"
	"pkg.jsn.cam/jsn/pkg/flagr"

	_ "net/http/pprof"
)

const Version = "0.1.0"

var (
	workers = flagr.Int("workers", "w", runtime.NumCPU()*2, "Number of worker goroutines")
	verbose = flagr.Bool("verbose", "v", true, "Verbose output")
	debug   = flagr.Bool("debug", "d", false, "Enable pprof profiling on port 6060")
	ignore  = flagr.String("ignore", "i", "", "Comma-separated list of paths/patterns to ignore")

	// Recording options
	interval = flagr.Int("interval", "", 30, "Scan interval in seconds")
	// 37387 = FSDVR (phone keypad mnemonic)
	webAddr  = flagr.String("web", "", "localhost:37387", "Start web UI on address (default localhost:37387, use 'off' to disable)")
	dbPath   = flagr.String("db", "", "", "Databsase path for session (default: ./session.db)")
	export   = flagr.Bool("export", "", false, "Export sessiosn to JSON")
	diffDirs = flagr.String("diff-dirs", "", "", "Comma-separated directories to compute diffs for (must be under root path)")
)

func main() {
	internal.HandleStartup()

	if *debug {
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	}

	args := flag.Args()

	// Handle export-only mode
	if *export {
		handleExport()
		return
	}

	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}

	rootPath := args[0]

	// Resolve absolute path
	absPath, err := filepath.Abs(rootPath)
	if err != nil {
		fmt.Printf("Error resolving path: %v\n", err)
		os.Exit(1)
	}

	// Determine database path
	dbFile := *dbPath
	if dbFile == "" {
		dbFile = "./session.db"
	}

	// Parse ignore patterns
	ignorePatterns := parseIgnorePatterns(*ignore)

	// Handle web UI address
	webAddress := *webAddr
	if webAddress == "off" || webAddress == "false" || webAddress == "0" {
		webAddress = ""
	}

	// Parse diff directories (only include dirs under root path)
	diffDirList := parseDiffDirs(*diffDirs, absPath)

	// Create session config
	config := &live.Config{
		RootPath:       absPath,
		Interval:       time.Duration(*interval) * time.Second,
		DBPath:         dbFile,
		Workers:        *workers,
		Verbose:        *verbose,
		IgnorePatterns: ignorePatterns,
		CaptureContent: true,
		WebAddr:        webAddress,
		DiffDirs:       diffDirList,
	}

	fmt.Printf("fsdvr v%s - Filesystem DVR\n", Version)
	fmt.Printf("Recording: %s\n", absPath)
	fmt.Printf("Interval: %ds | Workers: %d\n", *interval, *workers)
	fmt.Printf("Database: %s\n", dbFile)
	if webAddress != "" {
		fmt.Printf("Web UI: http://%s\n", webAddress)
	}
	if len(ignorePatterns) > 0 {
		fmt.Printf("Ignoring: %s\n", strings.Join(ignorePatterns, ", "))
	}
	fmt.Println()

	// Create session
	session, err := live.NewSession(config)
	if err != nil {
		fmt.Printf("Error creating session: %v\n", err)
		os.Exit(1)
	}
	defer session.Close()

	// Start recording
	if err := session.Start(context.Background()); err != nil {
		fmt.Printf("Error during recording: %v\n", err)
		os.Exit(1)
	}
}

func handleExport() {
	dbFile := *dbPath
	if dbFile == "" {
		fmt.Println("Error: -db path required for export")
		os.Exit(1)
	}

	store, err := live.OpenStore(dbFile)
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	fmt.Fprintf(os.Stderr, "Exporting session from: %s\n", dbFile)
	if err := store.ExportJSON(os.Stdout); err != nil {
		fmt.Printf("Error exporting: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf("fsdvr v%s - Filesystem DVR\n\n", Version)
	fmt.Println("Record filesystem changes over time. Like a screen recorder for your files.")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  fsdvr [options] <root_path>")
	fmt.Println("  fsdvr -export -db ./session.db")
	fmt.Println()
	fmt.Println("OPTIONS:")
	fmt.Printf("  -workers int    Number of parallel workers (default: %d)\n", runtime.NumCPU()*2)
	fmt.Println("  -v              Verbose output with progress")
	fmt.Println("  -d              Enable pprof profiling on port 6060")
	fmt.Println("  -ignore string  Comma-separated ignore patterns (e.g., '.cache,*.tmp')")
	fmt.Println()
	fmt.Println("RECORDING OPTIONS:")
	fmt.Println("  -interval int   Scan interval in seconds (default: 30)")
	fmt.Println("  -web addr       Start web UI on address (e.g., :8080)")
	fmt.Println("  -db path        Database path (default: ./session.db)")
	fmt.Println("  -export         Export session to JSON (writes to stdout)")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("  fsdvr /                             # Record root filesystem")
	fmt.Println("  fsdvr -interval 10 -web :8080 /     # With web UI, 10s interval")
	fmt.Println("  fsdvr -db ~/comp.db /home           # Custom database path")
	fmt.Println("  fsdvr -export -db ./session.db      # Export to JSON")
	fmt.Println()
	fmt.Println("WORKFLOW:")
	fmt.Println("  1. Start recording before competition/session")
	fmt.Println("  2. Do your work (changes are tracked)")
	fmt.Println("  3. Press Ctrl+C to stop")
	fmt.Println("  4. Export report or view web UI")
}

func parseIgnorePatterns(ignore string) []string {
	if ignore == "" {
		return nil
	}
	patterns := strings.Split(ignore, ",")
	result := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern != "" {
			result = append(result, pattern)
		}
	}
	return result
}

func parseDiffDirs(dirs string, rootPath string) []string {
	if dirs == "" {
		return nil
	}
	parts := strings.Split(dirs, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Resolve absolute path
		absDir, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		// Only include if under root path
		if strings.HasPrefix(absDir, rootPath) {
			result = append(result, absDir)
		} else {
			fmt.Printf("Warning: ignoring diff-dir %s (not under %s)\n", p, rootPath)
		}
	}
	return result
}
