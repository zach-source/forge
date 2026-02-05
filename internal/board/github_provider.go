package board

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/zach-source/forge/internal/github"
	"github.com/zach-source/forge/internal/mcp"
)

// GitHubProvider implements Provider for GitHub Projects.
type GitHubProvider struct {
	config *github.Config
}

// NewGitHubProvider creates a new GitHub provider.
func NewGitHubProvider() *GitHubProvider {
	return &GitHubProvider{}
}

// Name returns the provider name.
func (p *GitHubProvider) Name() string {
	return "github"
}

// DisplayName returns a human-readable name.
func (p *GitHubProvider) DisplayName() string {
	return "GitHub Projects"
}

// Icon returns an emoji icon for the provider.
func (p *GitHubProvider) Icon() string {
	return "🐙"
}

// MCPServers returns the MCP servers needed for GitHub.
func (p *GitHubProvider) MCPServers() []string {
	return []string{"github"}
}

// HasToken returns true if GITHUB_TOKEN is set.
func (p *GitHubProvider) HasToken() bool {
	return github.HasToken()
}

// TokenEnvVar returns the environment variable name.
func (p *GitHubProvider) TokenEnvVar() string {
	return "GITHUB_TOKEN"
}

// TokenSetupURL returns the URL for creating a GitHub token.
func (p *GitHubProvider) TokenSetupURL() string {
	return "https://github.com/settings/tokens"
}

// Load loads the GitHub configuration.
func (p *GitHubProvider) Load() error {
	cfg, err := github.Load()
	if err != nil {
		return err
	}
	p.config = cfg
	return nil
}

// Save saves the GitHub configuration.
func (p *GitHubProvider) Save() error {
	if p.config == nil {
		return fmt.Errorf("no configuration to save")
	}
	return github.Save(p.config)
}

// BoardIdentifier returns the project identifier.
func (p *GitHubProvider) BoardIdentifier() string {
	if p.config == nil {
		return ""
	}
	return fmt.Sprintf("%s/projects/%d", p.config.Owner, p.config.ProjectNumber)
}

// SyncPrompt returns the sync prompt for GitHub Projects.
func (p *GitHubProvider) SyncPrompt(oneShot bool) string {
	if p.config == nil {
		return ""
	}
	return GitHubSyncPrompt(p.config.Owner, p.config.ProjectNumber, p.config.Repo, oneShot)
}

// WatchPrompt returns the watch prompt for GitHub Projects.
func (p *GitHubProvider) WatchPrompt(interval int) string {
	if p.config == nil {
		return ""
	}
	return GitHubWatchPrompt(p.config.Owner, p.config.ProjectNumber, p.config.Repo, interval)
}

// CompletionPromise returns the promise text.
func (p *GitHubProvider) CompletionPromise() string {
	return GitHubCompletionPromise()
}

// Configure runs interactive configuration for GitHub.
func (p *GitHubProvider) Configure() error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("%s Forge %s Configuration\n", p.Icon(), p.DisplayName())
	fmt.Println()

	// Check for existing config
	existing, _ := github.Load()
	if existing != nil {
		fmt.Printf("Current project: %s\n", existing.ProjectURL())
		fmt.Println()
	}

	// Prompt for project URL or components
	fmt.Println("Enter your GitHub Project URL or details.")
	fmt.Println("URL formats supported:")
	fmt.Println("  https://github.com/users/USERNAME/projects/1")
	fmt.Println("  https://github.com/orgs/ORGNAME/projects/1")
	fmt.Println("  https://github.com/OWNER/REPO/projects/1")
	fmt.Println()
	fmt.Print("Project URL (or 'manual' for manual entry): ")

	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	input = strings.TrimSpace(input)

	if input == "manual" {
		// Manual entry
		fmt.Print("Owner (user or org): ")
		owner, _ := reader.ReadString('\n')
		owner = strings.TrimSpace(owner)

		fmt.Print("Repository (leave empty for user/org project): ")
		repo, _ := reader.ReadString('\n')
		repo = strings.TrimSpace(repo)

		fmt.Print("Project number: ")
		numStr, _ := reader.ReadString('\n')
		numStr = strings.TrimSpace(numStr)
		num, err := strconv.Atoi(numStr)
		if err != nil {
			return fmt.Errorf("invalid project number: %s", numStr)
		}

		p.config = &github.Config{
			Owner:         owner,
			Repo:          repo,
			ProjectNumber: num,
		}
	} else {
		// Parse URL
		owner, repo, num, err := github.ParseProjectURL(input)
		if err != nil {
			return err
		}
		p.config = &github.Config{
			Owner:         owner,
			Repo:          repo,
			ProjectNumber: num,
		}
	}

	if err := github.ValidateConfig(p.config.Owner, p.config.ProjectNumber); err != nil {
		return err
	}

	if err := p.Save(); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("✅ GitHub configuration saved")
	fmt.Printf("   Project: %s\n", p.config.ProjectURL())

	// Check for token
	if !p.HasToken() {
		fmt.Println()
		fmt.Printf("⚠️  %s not set!\n", p.TokenEnvVar())
		fmt.Printf("   Create a token at %s\n", p.TokenSetupURL())
		fmt.Println("   Required scopes: project, repo (for issue linking)")
		fmt.Printf("   Then: export %s=ghp_xxx\n", p.TokenEnvVar())
	}

	return nil
}

// ShowConfig displays the current GitHub configuration.
func (p *GitHubProvider) ShowConfig() error {
	if err := p.Load(); err != nil {
		return err
	}

	fmt.Printf("%s Forge %s Configuration\n", p.Icon(), p.DisplayName())
	fmt.Println()
	fmt.Printf("Owner:          %s\n", p.config.Owner)
	if p.config.Repo != "" {
		fmt.Printf("Repository:     %s\n", p.config.Repo)
	}
	fmt.Printf("Project Number: %d\n", p.config.ProjectNumber)
	fmt.Printf("Project URL:    %s\n", p.config.ProjectURL())
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
