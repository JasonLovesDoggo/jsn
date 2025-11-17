package skit

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
)

// ScriptType enumerates the supported script behaviors.
type ScriptType string

const (
	// ScriptTypeRun execs a single command.
	ScriptTypeRun ScriptType = "run"
	// ScriptTypeToggle flips between enable/disable commands.
	ScriptTypeToggle ScriptType = "toggle"
)

// Command describes either a script path or an inline shell snippet.
type Command struct {
	Value  string
	Inline bool
}

// CommandMap stores per-platform commands with an optional default.
type CommandMap map[string]Command

// CommandFor resolves the best command for the provided OS, falling back to "default".
func (m CommandMap) CommandFor(goos string) (Command, error) {
	if len(m) == 0 {
		return Command{}, errors.New("no commands configured")
	}
	if goos != "" {
		goos = strings.ToLower(goos)
		if cmd, ok := m[goos]; ok && cmd.Value != "" {
			return cmd, nil
		}
	}
	if cmd, ok := m["default"]; ok && cmd.Value != "" {
		return cmd, nil
	}
	return Command{}, fmt.Errorf("no command available for %q (missing %q fallback)", goos, "default")
}

// ToggleSpec stores the enable/disable commands for a toggle script.
type ToggleSpec struct {
	Enable  CommandMap `toml:"enable"`
	Disable CommandMap `toml:"disable"`
}

// Script describes a runnable entry.
type Script struct {
	Slug        string
	Dir         string
	ConfigPath  string
	Name        string   `toml:"name"`
	Description string   `toml:"description"`
	Tags        []string `toml:"tags"`
	Type        ScriptType
	Platforms   []string          `toml:"platforms"`
	StateHint   string            `toml:"state_hint"`
	Env         map[string]string `toml:"env"`

	Exec   CommandMap `toml:"exec"`
	Toggle ToggleSpec `toml:"toggle"`
}

// SupportsCurrentPlatform returns true when the script is available for this OS.
func (s *Script) SupportsCurrentPlatform() bool {
	if len(s.Platforms) == 0 {
		return true
	}
	current := runtime.GOOS
	for _, p := range s.Platforms {
		if strings.EqualFold(p, current) {
			return true
		}
	}
	return false
}

// ResolveCommand returns the command path for the script based on type and action.
func (s *Script) ResolveCommand(action ToggleAction) (Command, error) {
	switch s.Type {
	case ScriptTypeRun:
		return s.Exec.CommandFor(runtime.GOOS)
	case ScriptTypeToggle:
		switch action {
		case ToggleActionEnable:
			return s.Toggle.Enable.CommandFor(runtime.GOOS)
		case ToggleActionDisable:
			return s.Toggle.Disable.CommandFor(runtime.GOOS)
		default:
			return Command{}, fmt.Errorf("unknown toggle action %q", action)
		}
	default:
		return Command{}, fmt.Errorf("unsupported script type %q", s.Type)
	}
}

// ToggleAction identifies which half of a toggle script is executed.
type ToggleAction string

const (
	// ToggleActionEnable runs the enable command.
	ToggleActionEnable ToggleAction = "enable"
	// ToggleActionDisable runs the disable command.
	ToggleActionDisable ToggleAction = "disable"
)

// NextToggleAction flips enable/disable based on last recorded action.
func NextToggleAction(last ToggleAction) ToggleAction {
	if last == ToggleActionEnable {
		return ToggleActionDisable
	}
	return ToggleActionEnable
}

// StateHintOrDefault returns a friendly label for toggle state displays.
func (s *Script) StateHintOrDefault() string {
	if s.StateHint != "" {
		return s.StateHint
	}
	return s.Name
}
