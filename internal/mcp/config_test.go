package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestConfiguratorGenerate(t *testing.T) {
	c := NewConfigurator().WithServers([]string{"graphiti", "context7"})

	path, err := c.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	defer os.Remove(path)

	// Read and parse the generated config
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Reading generated config: %v", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("Parsing generated config: %v", err)
	}

	// Check that requested servers are present
	if _, ok := config.MCPServers["graphiti"]; !ok {
		t.Error("Expected graphiti server in config")
	}
	if _, ok := config.MCPServers["context7"]; !ok {
		t.Error("Expected context7 server in config")
	}
}

func TestConfiguratorWithBaseConfig(t *testing.T) {
	// Create a temp base config
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "base.json")

	baseConfig := Config{
		MCPServers: map[string]Server{
			"custom-server": {
				Type:    "stdio",
				Command: "custom-cmd",
				Args:    []string{"--custom"},
			},
		},
	}

	data, _ := json.Marshal(baseConfig)
	if err := os.WriteFile(basePath, data, 0644); err != nil {
		t.Fatal(err)
	}

	c := NewConfigurator().
		WithBaseConfig(basePath).
		WithServers([]string{"graphiti"})

	path, err := c.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	defer os.Remove(path)

	// Read and parse
	data, _ = os.ReadFile(path)
	var config Config
	json.Unmarshal(data, &config)

	// Should have both custom and graphiti
	if _, ok := config.MCPServers["custom-server"]; !ok {
		t.Error("Expected custom-server from base config")
	}
	if _, ok := config.MCPServers["graphiti"]; !ok {
		t.Error("Expected graphiti server")
	}
}

func TestParseServerList(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"graphiti,context7", []string{"graphiti", "context7"}},
		{"graphiti, context7, sequential-thinking", []string{"graphiti", "context7", "sequential-thinking"}},
		{"", nil},
		{"single", []string{"single"}},
	}

	for _, tt := range tests {
		got := ParseServerList(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("ParseServerList(%q) = %v, want %v", tt.input, got, tt.expected)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("ParseServerList(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.expected[i])
			}
		}
	}
}
