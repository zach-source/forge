package board

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/zach-source/forge/internal/mcp"
	"github.com/zach-source/forge/internal/notion"
)

// NotionProvider implements Provider for Notion databases.
type NotionProvider struct {
	config *notion.Config
}

// NewNotionProvider creates a new Notion provider.
func NewNotionProvider() *NotionProvider {
	return &NotionProvider{}
}

// Name returns the provider name.
func (p *NotionProvider) Name() string {
	return "notion"
}

// DisplayName returns a human-readable name.
func (p *NotionProvider) DisplayName() string {
	return "Notion"
}

// Icon returns an emoji icon for the provider.
func (p *NotionProvider) Icon() string {
	return "🗂️"
}

// MCPServers returns the MCP servers needed for Notion.
func (p *NotionProvider) MCPServers() []string {
	return []string{"notion"}
}

// HasToken returns true if NOTION_API_TOKEN is set.
func (p *NotionProvider) HasToken() bool {
	return notion.HasToken()
}

// TokenEnvVar returns the environment variable name.
func (p *NotionProvider) TokenEnvVar() string {
	return "NOTION_API_TOKEN"
}

// TokenSetupURL returns the URL for creating a Notion integration.
func (p *NotionProvider) TokenSetupURL() string {
	return "https://www.notion.so/my-integrations"
}

// Load loads the Notion configuration.
func (p *NotionProvider) Load() error {
	cfg, err := notion.Load()
	if err != nil {
		return err
	}
	p.config = cfg
	return nil
}

// Save saves the Notion configuration.
func (p *NotionProvider) Save() error {
	if p.config == nil {
		return fmt.Errorf("no configuration to save")
	}
	return notion.Save(p.config)
}

// BoardIdentifier returns the database ID.
func (p *NotionProvider) BoardIdentifier() string {
	if p.config == nil {
		return ""
	}
	return p.config.DatabaseID
}

// SyncPrompt returns the sync prompt for Notion.
func (p *NotionProvider) SyncPrompt(oneShot bool) string {
	if p.config == nil {
		return ""
	}
	return SyncPrompt(p.config.DatabaseID, oneShot)
}

// WatchPrompt returns the watch prompt for Notion.
func (p *NotionProvider) WatchPrompt(interval int) string {
	if p.config == nil {
		return ""
	}
	return WatchPrompt(p.config.DatabaseID, interval)
}

// CompletionPromise returns the promise text.
func (p *NotionProvider) CompletionPromise() string {
	return CompletionPromise()
}

// Configure runs interactive configuration for Notion.
func (p *NotionProvider) Configure() error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("%s Forge %s Configuration\n", p.Icon(), p.DisplayName())
	fmt.Println()

	// Check for existing config
	existing, _ := notion.Load()
	if existing != nil {
		fmt.Printf("Current database ID: %s\n", existing.DatabaseID)
		fmt.Println()
	}

	// Prompt for database ID
	fmt.Println("Enter your Notion database ID.")
	fmt.Println("You can find this in the database URL:")
	fmt.Println("  https://www.notion.so/workspace/<database-id>?v=...")
	fmt.Println()
	fmt.Print("Database ID: ")

	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	databaseID := strings.TrimSpace(input)
	if err := notion.ValidateDatabaseID(databaseID); err != nil {
		return err
	}

	// Save config
	p.config = &notion.Config{
		DatabaseID: databaseID,
	}
	if err := p.Save(); err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("✅ Configuration saved to %s\n", notion.ConfigPath())

	// Check for token
	if !p.HasToken() {
		fmt.Println()
		fmt.Printf("⚠️  %s not set!\n", p.TokenEnvVar())
		fmt.Printf("   Create an integration at %s\n", p.TokenSetupURL())
		fmt.Printf("   Then: export %s=secret_xxx\n", p.TokenEnvVar())
	}

	return nil
}

// ShowConfig displays the current Notion configuration.
func (p *NotionProvider) ShowConfig() error {
	if err := p.Load(); err != nil {
		return err
	}

	fmt.Printf("%s Forge %s Configuration\n", p.Icon(), p.DisplayName())
	fmt.Println()
	fmt.Printf("Config file:  %s\n", notion.ConfigPath())
	fmt.Printf("Database ID:  %s\n", p.config.DatabaseID)
	if p.config.BoardID != "" {
		fmt.Printf("Board ID:     %s\n", p.config.BoardID)
	}
	fmt.Println()

	if p.HasToken() {
		fmt.Printf("✅ %s is set\n", p.TokenEnvVar())
	} else {
		fmt.Printf("❌ %s is not set\n", p.TokenEnvVar())
	}

	// Show available MCP servers
	fmt.Println()
	fmt.Println("Available MCP servers:")
	for _, name := range mcp.AvailableServers() {
		marker := "  "
		for _, s := range p.MCPServers() {
			if name == s {
				marker = "• "
				break
			}
		}
		fmt.Printf("  %s%s\n", marker, name)
	}

	return nil
}
