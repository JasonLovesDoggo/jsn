package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

var errConfigMissing = errors.New("config missing")

type Config struct {
	DefaultProfile string             `toml:"default_profile"`
	Profiles       map[string]Profile `toml:"profile"`
}

type Profile struct {
	APIKey     string `toml:"api_key"`
	Region     string `toml:"region"`
	BaseURL    string `toml:"base_url"`
	Namespace  string `toml:"namespace"`
	VectorAttr string `toml:"vector_attr"`
	TextAttr   string `toml:"text_attr"`
	TopK       int    `toml:"top_k"`
}

func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "./config.toml"
	}
	return filepath.Join(home, ".config", "tpuf", "config.toml")
}

func LoadConfig(path string) (*Config, error) {
	cfg := &Config{}
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", errConfigMissing, path)
		}
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	cfg.applyDefaults()
	return cfg, nil
}

func LoadOrInitConfig(path string) (*Config, bool, error) {
	cfg, err := LoadConfig(path)
	if err == nil {
		return cfg, false, nil
	}
	if !errors.Is(err, errConfigMissing) {
		return nil, false, err
	}
	cfg = DefaultConfigFromEnv()
	cfg.applyDefaults()
	return cfg, false, nil
}

func (c *Config) applyDefaults() {
	if c.Profiles == nil {
		c.Profiles = make(map[string]Profile)
	}
	for key, prof := range c.Profiles {
		if prof.VectorAttr == "" {
			prof.VectorAttr = "vector"
		}
		if prof.TextAttr == "" {
			prof.TextAttr = "text"
		}
		if prof.TopK == 0 {
			prof.TopK = 20
		}
		c.Profiles[key] = prof
	}
}

func (c *Config) ResolveProfile(name string) (Profile, string, error) {
	if len(c.Profiles) == 0 {
		return Profile{}, "", errors.New("no profiles found in config")
	}
	if name == "" {
		if c.DefaultProfile != "" {
			name = c.DefaultProfile
		} else if len(c.Profiles) == 1 {
			for key := range c.Profiles {
				name = key
				break
			}
		}
	}
	if name == "" {
		return Profile{}, "", errors.New("profile required (set default_profile or pass --profile)")
	}
	prof, ok := c.Profiles[name]
	if !ok {
		return Profile{}, "", fmt.Errorf("profile %q not found", name)
	}
	return prof, name, nil
}

func (c *Config) ProfileNames() []string {
	names := make([]string, 0, len(c.Profiles))
	for name := range c.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (c *Config) ConfigSummary(active string) string {
	if active == "" {
		return "no profile"
	}
	return fmt.Sprintf("profile:%s", active)
}

func (p Profile) DisplayRegion() string {
	if p.Region == "" {
		return "region:?"
	}
	return fmt.Sprintf("region:%s", strings.TrimSpace(p.Region))
}

func needsOnboarding(profile Profile) bool {
	if strings.TrimSpace(profile.APIKey) == "" {
		return true
	}
	if strings.TrimSpace(profile.BaseURL) == "" && strings.TrimSpace(profile.Region) == "" {
		return true
	}
	return false
}

func DefaultConfigFromEnv() *Config {
	profile := Profile{
		APIKey:     strings.TrimSpace(os.Getenv("TURBOPUFFER_API_KEY")),
		Region:     strings.TrimSpace(os.Getenv("TURBOPUFFER_REGION")),
		BaseURL:    strings.TrimSpace(os.Getenv("TURBOPUFFER_BASE_URL")),
		Namespace:  strings.TrimSpace(os.Getenv("TURBOPUFFER_NAMESPACE")),
		VectorAttr: "vector",
		TextAttr:   "text",
		TopK:       20,
	}
	return &Config{
		DefaultProfile: "default",
		Profiles: map[string]Profile{
			"default": profile,
		},
	}
}

func WriteConfig(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	encoder := toml.NewEncoder(&buf)
	if err := encoder.Encode(cfg); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o600)
}

func ConfigTemplate(path string, profile Profile) string {
	apiKey := profile.APIKey
	if apiKey == "" {
		apiKey = "tp_..."
	}
	region := profile.Region
	if region == "" {
		region = "gcp-us-central1"
	}
	namespace := profile.Namespace
	if namespace == "" {
		namespace = "my_namespace"
	}
	return fmt.Sprintf(`# tpuf config
# path: %s
# You can also skip this file and rely on TURBOPUFFER_API_KEY/TURBOPUFFER_REGION.

default_profile = "default"

[profile.default]
api_key = "%s"
region = "%s"
namespace = "%s"
vector_attr = "vector"
text_attr = "text"
top_k = 20

#[profile.staging]
#api_key = "tp_..."
#region = "gcp-us-east4"
#namespace = "my_namespace_staging"
`, path, apiKey, region, namespace)
}
