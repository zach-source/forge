// Package mcp provides MCP server configuration for forge agents.
package mcp

// Server represents an MCP server configuration.
type Server struct {
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

// DefaultServers returns the default MCP servers for forge agents.
func DefaultServers() map[string]Server {
	return map[string]Server{
		"graphiti": {
			Type:    "stdio",
			Command: "npx",
			Args:    []string{"-y", "mcp-remote", "http://localhost:51847/mcp/"},
		},
		"context7": {
			Type:    "stdio",
			Command: "npx",
			Args:    []string{"-y", "@context7/mcp"},
		},
		"sequential-thinking": {
			Type:    "stdio",
			Command: "npx",
			Args:    []string{"-y", "@anthropic/mcp-sequential-thinking"},
		},
		"notion": {
			Type:    "stdio",
			Command: "npx",
			Args:    []string{"-y", "@notionhq/notion-mcp-server"},
			Env: map[string]string{
				"OPENAPI_MCP_HEADERS": `{"Authorization": "Bearer ${NOTION_API_TOKEN}", "Notion-Version": "2022-06-28"}`,
			},
		},
		"github": {
			Type:    "stdio",
			Command: "npx",
			Args:    []string{"-y", "@anthropic/github-mcp-server"},
			Env: map[string]string{
				"GITHUB_PERSONAL_ACCESS_TOKEN": "${GITHUB_TOKEN}",
			},
		},
	}
}

// GetServer returns a server configuration by name, or nil if not found.
func GetServer(name string) *Server {
	servers := DefaultServers()
	if s, ok := servers[name]; ok {
		return &s
	}
	return nil
}

// AvailableServers returns the names of all available default servers.
func AvailableServers() []string {
	return []string{"graphiti", "context7", "sequential-thinking", "notion", "github"}
}
