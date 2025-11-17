package skit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	// DefaultScriptsDir is where the repository stores user scripts.
	DefaultScriptsDir = "var/skit/scripts"
	// DefaultStateDir holds per-script toggle state.
	DefaultStateDir = "var/skit/state"
	// ConfigFileName is the manifest inside each script directory.
	ConfigFileName = "skit.toml"
)

// LoadScripts parses every script manifest under root and returns the normalized list.
func LoadScripts(root string) ([]*Script, error) {
	root = filepath.Clean(root)
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(rootAbs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if mkErr := os.MkdirAll(rootAbs, 0o755); mkErr != nil {
				return nil, mkErr
			}
			return nil, nil
		}
		return nil, err
	}
	var scripts []*Script
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(rootAbs, entry.Name())
		cfgPath := filepath.Join(dir, ConfigFileName)
		data, err := os.ReadFile(cfgPath)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", cfgPath, err)
		}
		raw, err := parseConfig(data)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", cfgPath, err)
		}
		s, err := normalizeScript(entry.Name(), dir, cfgPath, raw)
		if err != nil {
			return nil, fmt.Errorf("validating %s: %w", cfgPath, err)
		}
		scripts = append(scripts, s)
	}
	sort.Slice(scripts, func(i, j int) bool {
		return scripts[i].Name < scripts[j].Name
	})
	return scripts, nil
}

type configFile struct {
	Name        string            `toml:"name"`
	Description string            `toml:"description"`
	Tags        []string          `toml:"tags"`
	Type        string            `toml:"type"`
	Platforms   []string          `toml:"platforms"`
	Env         map[string]string `toml:"env"`
	StateHint   string            `toml:"state_hint"`
	Exec        map[string]string `toml:"exec"`
	Toggle      struct {
		Enable  map[string]string `toml:"enable"`
		Disable map[string]string `toml:"disable"`
	} `toml:"toggle"`
}

func parseConfig(data []byte) (*configFile, error) {
	var cfg configFile
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func normalizeScript(slug, dir, cfgPath string, cfg *configFile) (*Script, error) {
	if cfg.Name == "" {
		return nil, errors.New("name is required")
	}
	s := &Script{
		Slug:        slug,
		Dir:         dir,
		ConfigPath:  cfgPath,
		Name:        cfg.Name,
		Description: cfg.Description,
		Tags:        append([]string(nil), cfg.Tags...),
		Platforms:   append([]string(nil), cfg.Platforms...),
		Env:         cloneStringMap(cfg.Env),
		StateHint:   cfg.StateHint,
	}
	if cfg.Type == "" {
		s.Type = ScriptTypeRun
	} else {
		s.Type = ScriptType(cfg.Type)
	}
	switch s.Type {
	case ScriptTypeRun:
		if len(cfg.Exec) == 0 {
			return nil, errors.New("exec block is required for run scripts")
		}
		cmdMap, err := buildCommandMap(dir, cfg.Exec)
		if err != nil {
			return nil, err
		}
		s.Exec = cmdMap
	case ScriptTypeToggle:
		if len(cfg.Toggle.Enable) == 0 || len(cfg.Toggle.Disable) == 0 {
			return nil, errors.New("toggle.enable and toggle.disable must be provided for toggle scripts")
		}
		enable, err := buildCommandMap(dir, cfg.Toggle.Enable)
		if err != nil {
			return nil, fmt.Errorf("enable: %w", err)
		}
		disable, err := buildCommandMap(dir, cfg.Toggle.Disable)
		if err != nil {
			return nil, fmt.Errorf("disable: %w", err)
		}
		s.Toggle = ToggleSpec{Enable: enable, Disable: disable}
	default:
		return nil, fmt.Errorf("unsupported script type %q", s.Type)
	}
	return s, nil
}

func buildCommandMap(dir string, raw map[string]string) (CommandMap, error) {
	if len(raw) == 0 {
		return nil, errors.New("command map is empty")
	}
	cmds := make(CommandMap, len(raw))
	for key, value := range raw {
		val := strings.TrimSpace(value)
		if val == "" {
			return nil, fmt.Errorf("%s: command is empty", key)
		}
		command := Command{}
		if strings.HasPrefix(val, "!") {
			command.Inline = true
			command.Value = strings.TrimSpace(strings.TrimPrefix(val, "!"))
			if command.Value == "" {
				return nil, fmt.Errorf("%s: inline command is empty", key)
			}
		} else {
			resolved := val
			if !filepath.IsAbs(val) {
				resolved = filepath.Join(dir, val)
			}
			if err := validateExecutable(resolved); err != nil {
				return nil, fmt.Errorf("%s: %w", key, err)
			}
			command.Value = resolved
		}
		cmds[strings.ToLower(key)] = command
	}
	return cmds, nil
}

func validateExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory, not an executable", path)
	}
	if runtime.GOOS != "windows" {
		mode := info.Mode()
		if mode&0111 == 0 {
			newMode := mode | 0o111
			if err := os.Chmod(path, newMode); err != nil {
				return fmt.Errorf("%s is not marked executable", path)
			}
		}
	}
	return nil
}

func cloneStringMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
