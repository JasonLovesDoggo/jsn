package main

import (
	"log"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"pkg.jsn.cam/jsn/internal/skit"
)

func main() {
	scriptsDir, err := filepath.Abs(skit.DefaultScriptsDir)
	if err != nil {
		log.Fatal(err)
	}
	stateDir, err := filepath.Abs(skit.DefaultStateDir)
	if err != nil {
		log.Fatal(err)
	}
	scripts, err := skit.LoadScripts(scriptsDir)
	if err != nil {
		log.Fatal(err)
	}
	stateStore, err := skit.NewStateStore(stateDir)
	if err != nil {
		log.Fatal(err)
	}
	runner := skit.NewRunner(stateStore)
	m := newModel(scriptsDir, scripts, runner, stateStore)
	if err := tea.NewProgram(m, tea.WithAltScreen()).Start(); err != nil {
		log.Fatal(err)
	}
}
