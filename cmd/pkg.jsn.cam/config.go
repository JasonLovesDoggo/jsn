package main

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config represents the main application configuration
type Config struct {
	// Only unmarshal the "repo" table into this struct
	Repo map[string]RepoProvider `toml:"repo"`
	// This will be filled manually after parsing
	Repos map[string]RepoConfig
}

// RepoProvider defines a source of repositories (e.g., GitHub, GitLab)
type RepoProvider struct {
	Username string `toml:"username"`
	URL      string `toml:"url"`
	Default  bool   `toml:"default"`
}

// RepoConfig defines a specific repository configuration
type RepoConfig struct {
	Type        string `toml:"type"`
	Description string `toml:"desc"`
}

// LoadConfig loads and parses the TOML configuration file
func LoadConfig(path string, lg *slog.Logger) (*Config, error) {
	// Initialize our config with empty maps
	config := &Config{
		Repo:  make(map[string]RepoProvider),
		Repos: make(map[string]RepoConfig),
	}

	// Decode the TOML file with metadata to handle undecoded keys
	var tmp struct {
		Repo map[string]RepoProvider `toml:"repo"`
	}

	_, err := DecodeFile(path, &tmp)
	if err != nil {
		return nil, err
	}

	// Copy the decoded repo section
	config.Repo = tmp.Repo

	// Process all top-level tables that aren't "repo"
	rawData := map[string]interface{}{}
	if _, err := DecodeFile(path, &rawData); err != nil {
		return nil, err
	}

	// Iterate through the raw data and extract repo configs
	for key, value := range rawData {
		// Skip the "repo" table which we already processed
		if key == "repo" {
			continue
		}

		// Process each table as a repo config
		if tableData, ok := value.(map[string]interface{}); ok {
			var repoConfig RepoConfig

			// Extract type and description
			if typeStr, ok := tableData["type"].(string); ok {
				repoConfig.Type = typeStr
			}

			if desc, ok := tableData["desc"].(string); ok {
				repoConfig.Description = desc
			}

			// Add to the repos map
			config.Repos[key] = repoConfig
		}
	}

	return config, nil
}

// BuildRepos builds the list of Repo objects from the configuration
func BuildRepos(config *Config, lg *slog.Logger) []Repo {
	var repos []Repo

	// Find default repo provider if needed
	var defaultProvider string
	for providerName, provider := range config.Repo {
		if provider.Default {
			defaultProvider = providerName
			break
		}
	}

	// Debug logging for config
	lg.Debug("config loaded",
		"providers", len(config.Repo),
		"projects", len(config.Repos),
		"default", defaultProvider)

	// Process each repository configuration
	for slug, repoConfig := range config.Repos {
		repoType := repoConfig.Type

		// If type is not specified, use default provider
		if repoType == "" {
			if defaultProvider == "" {
				lg.Error("no type specified for repo and no default provider set", "slug", slug)
				continue
			}
			repoType = defaultProvider
		}

		provider, ok := config.Repo[repoType]
		if !ok {
			lg.Error("unknown repo provider", "type", repoType, "slug", slug)
			continue
		}

		repos = append(repos, Repo{
			Kind:        repoType,
			Domain:      getRepoDomain(provider.URL),
			User:        provider.Username,
			Repo:        slug,
			Description: repoConfig.Description,
		})
	}

	return repos
}

// getRepoDomain extracts the domain from a URL
func getRepoDomain(url string) string {
	return strings.TrimPrefix(url, "https://")
}

// Simple helper to convert a map to TOML string for re-parsing
// This is a workaround for limitations in how we can decode specific sections
func tomlString(m map[string]interface{}) string {
	var result strings.Builder
	encodeMap(&result, m, "")
	return result.String()
}

func encodeMap(b *strings.Builder, m map[string]interface{}, prefix string) {
	for k, v := range m {
		fullKey := k
		if prefix != "" {
			fullKey = prefix + "." + k
		}

		switch val := v.(type) {
		case map[string]interface{}:
			b.WriteString("[" + fullKey + "]\n")
			for subK, subV := range val {
				encodeValue(b, subK, subV)
			}
		default:
			encodeValue(b, fullKey, v)
		}
	}
}

func encodeValue(b *strings.Builder, key string, value interface{}) {
	b.WriteString(key + " = ")

	switch v := value.(type) {
	case string:
		b.WriteString("\"" + v + "\"")
	case int, int64, float64, bool:
		b.WriteString(stringify(v))
	}

	b.WriteString("\n")
}

func stringify(v interface{}) string {
	return fmt.Sprintf("%v", v)
}
func DecodeFile(path string, v any) (toml.MetaData, error) {
	fp, err := Open(path)
	if err != nil {
		return toml.MetaData{}, err
	}
	defer fp.Close()
	return toml.NewDecoder(fp).Decode(v)
}
