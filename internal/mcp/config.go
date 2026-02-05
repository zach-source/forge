package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config represents an MCP configuration file.
type Config struct {
	MCPServers map[string]Server `json:"mcpServers"`
}

// Configurator generates MCP configuration files for forge agents.
type Configurator struct {
	BaseConfigPath string            // Path to user's base .mcp.json
	ExtraServers   map[string]Server // Additional servers to include
	SelectedNames  []string          // Names of default servers to include
}

// NewConfigurator creates a new MCP configurator.
func NewConfigurator() *Configurator {
	return &Configurator{
		ExtraServers:  make(map[string]Server),
		SelectedNames: []string{}, // Empty default - MCP servers should be explicitly requested
	}
}

// WithBaseConfig sets the path to the user's base MCP config.
func (c *Configurator) WithBaseConfig(path string) *Configurator {
	c.BaseConfigPath = path
	return c
}

// WithServers sets which default servers to include by name.
func (c *Configurator) WithServers(names []string) *Configurator {
	c.SelectedNames = names
	return c
}

// WithExtraServer adds an extra server configuration.
func (c *Configurator) WithExtraServer(name string, server Server) *Configurator {
	c.ExtraServers[name] = server
	return c
}

// Generate creates a temporary MCP config file and returns its path.
// The caller is responsible for cleaning up the file.
func (c *Configurator) Generate() (string, error) {
	config := Config{
		MCPServers: make(map[string]Server),
	}

	// Load base config if specified
	if c.BaseConfigPath != "" {
		baseConfig, err := c.loadBaseConfig()
		if err != nil {
			// Non-fatal, just skip
			fmt.Printf("Warning: failed to load base MCP config: %v\n", err)
		} else {
			for name, server := range baseConfig.MCPServers {
				config.MCPServers[name] = server
			}
		}
	}

	// Add selected default servers
	defaults := DefaultServers()
	for _, name := range c.SelectedNames {
		if server, ok := defaults[name]; ok {
			config.MCPServers[name] = server
		}
	}

	// Add extra servers (override if exists)
	for name, server := range c.ExtraServers {
		config.MCPServers[name] = server
	}

	// Generate temp file
	tmpDir := os.TempDir()
	tmpFile, err := os.CreateTemp(tmpDir, "forge-mcp-*.json")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	defer func() { _ = tmpFile.Close() }()

	encoder := json.NewEncoder(tmpFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(config); err != nil {
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("encoding config: %w", err)
	}

	return tmpFile.Name(), nil
}

// loadBaseConfig loads a base MCP configuration file.
func (c *Configurator) loadBaseConfig() (*Config, error) {
	data, err := os.ReadFile(c.BaseConfigPath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// ParseServerList parses a comma-separated list of server names.
func ParseServerList(list string) []string {
	if list == "" {
		return nil
	}

	parts := strings.Split(list, ",")
	var names []string
	for _, p := range parts {
		name := strings.TrimSpace(p)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

// FindUserMCPConfig tries to find the user's MCP config file.
func FindUserMCPConfig() string {
	// Check common locations
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".mcp.json"),
		filepath.Join(home, ".config", "mcp", "config.json"),
		".mcp.json",
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}
