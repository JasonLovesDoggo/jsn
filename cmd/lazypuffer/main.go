package main

import (
	"fmt"
	"log"

	tea "github.com/charmbracelet/bubbletea"

	"pkg.jsn.cam/jsn/pkg/flagr"
)

func main() {
	flags := flagr.New("lazypuffer")
	configPath := flags.String("config", "c", "", "config file path (default: ~/.config/lazypuffer/config.toml)")
	profileName := flags.String("profile", "p", "", "profile name to use")
	namespaceOverride := flags.String("namespace", "n", "", "override default namespace")
	help := flags.Bool("help", "h", false, "show help")
	flags.Parse()

	if *help {
		flags.Usage()
		return
	}

	path := *configPath
	if path == "" {
		path = defaultConfigPath()
	}

	cfg, _, err := LoadOrInitConfig(path)
	if err != nil {
		m := newSetupModel(path, err)
		if err := tea.NewProgram(m, tea.WithAltScreen()).Start(); err != nil {
			log.Fatalf("setup: %v", err)
		}
		return
	}

	profile, profileKey, err := cfg.ResolveProfile(*profileName)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if *namespaceOverride != "" {
		profile.Namespace = *namespaceOverride
	}

	startupStatus := ""
	if needsOnboarding(profile) {
		updatedProfile, saved, err := runOnboarding(path, profileKey, cfg, profile)
		if err != nil {
			log.Fatalf("onboarding: %v", err)
		}
		if updatedProfile == nil {
			return
		}
		profile = *updatedProfile
		if saved {
			startupStatus = fmt.Sprintf("Saved config to %s", path)
		}
	}

	m := NewModel(cfg, path, profileKey, profile)
	if startupStatus != "" {
		m.setStatus(startupStatus, false)
	}
	if err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Start(); err != nil {
		log.Fatalf("tui: %v", err)
	}
	fmt.Println()
}
