// Package notion provides Notion board configuration for forge.
package notion

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents Notion board configuration.
type Config struct {
	BoardID    string `yaml:"board_id"`
	DatabaseID string `yaml:"database_id"`
}

// FullConfig represents the full forge config file with Notion section.
type FullConfig struct {
	Notion Config `yaml:"notion"`
}

// configPaths returns the paths to check for config files.
func configPaths() []string {
	home, _ := os.UserHomeDir()
	return []string{
		filepath.Join(home, ".forge", "config.yaml"),
		filepath.Join(home, ".foundry", "config.yaml"),
	}
}

// ConfigPath returns the path to the config file (first existing or default).
func ConfigPath() string {
	for _, path := range configPaths() {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	// Default to ~/.forge/config.yaml
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".forge", "config.yaml")
}

// Load loads Notion configuration from the config file.
func Load() (*Config, error) {
	for _, path := range configPaths() {
		cfg, err := loadFromPath(path)
		if err == nil {
			return cfg, nil
		}
		// Only continue if file doesn't exist
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading config from %s: %w", path, err)
		}
	}
	return nil, fmt.Errorf("no config file found; run 'forge board --config' to set up")
}

// loadFromPath loads config from a specific path.
func loadFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var full FullConfig
	if err := yaml.Unmarshal(data, &full); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if full.Notion.DatabaseID == "" {
		return nil, fmt.Errorf("notion.database_id not configured")
	}

	return &full.Notion, nil
}

// Save saves Notion configuration to the config file.
func Save(cfg *Config) error {
	configPath := ConfigPath()

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	// Load existing config or create new
	var full FullConfig
	if data, err := os.ReadFile(configPath); err == nil {
		_ = yaml.Unmarshal(data, &full) // Ignore errors, start fresh if invalid
	}

	// Update Notion config
	full.Notion = *cfg

	// Write back
	data, err := yaml.Marshal(&full)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// HasToken returns true if NOTION_API_TOKEN is set in the environment.
func HasToken() bool {
	return os.Getenv("NOTION_API_TOKEN") != ""
}

// ValidateDatabaseID does basic validation of a Notion database ID.
func ValidateDatabaseID(id string) error {
	if id == "" {
		return fmt.Errorf("database ID cannot be empty")
	}
	// Notion database IDs are 32 character hex strings (with optional dashes)
	// They can be extracted from URLs like:
	// https://www.notion.so/workspace/<database-id>?v=...
	if len(id) < 32 {
		return fmt.Errorf("database ID appears too short (expected 32+ characters)")
	}
	return nil
}
