package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"pkg.jsn.cam/jsn/internal/skit"
)

func main() {
	p, err := resolvePaths()
	if err != nil {
		log.Fatal(err)
	}
	repoURL := os.Getenv("SKIT_SYNC_REPO")
	branch := os.Getenv("SKIT_SYNC_BRANCH")
	if repoURL != "" {
		if err := ensureGitRepo(p.ScriptsDir, repoURL, branch); err != nil {
			log.Fatalf("git sync: %v", err)
		}
	} else {
		seedScriptsIfEmpty(p.ScriptsDir)
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "sync":
			if repoURL == "" {
				log.Fatal("SKIT_SYNC_REPO must be set to use `sk sync`")
			}
			if err := gitSyncChanges(p.ScriptsDir, branch); err != nil {
				log.Fatalf("git sync: %v", err)
			}
			fmt.Println("Scripts synced with origin.")
			return
		default:
			log.Fatalf("unknown command %q", os.Args[1])
		}
	}

	scripts, err := skit.LoadScripts(p.ScriptsDir)
	if err != nil {
		log.Fatal(err)
	}
	stateStore, err := skit.NewStateStore(p.StateDir)
	if err != nil {
		log.Fatal(err)
	}

	m := newModel(p.ScriptsDir, scripts, skit.NewRunner(stateStore), stateStore)
	if err := tea.NewProgram(m, tea.WithAltScreen()).Start(); err != nil {
		log.Fatal(err)
	}
}
