package board

// Provider defines the interface for external project management systems.
// Implementations include Notion, GitHub Projects, and potentially others
// like Linear, Jira, Todoist, etc.
type Provider interface {
	// Name returns the provider name (e.g., "notion", "github")
	Name() string

	// DisplayName returns a human-readable name (e.g., "Notion", "GitHub Projects")
	DisplayName() string

	// Icon returns an emoji icon for the provider
	Icon() string

	// MCPServers returns the MCP servers needed for this provider
	MCPServers() []string

	// HasToken returns true if the required auth token is set
	HasToken() bool

	// TokenEnvVar returns the environment variable name for the auth token
	TokenEnvVar() string

	// TokenSetupURL returns the URL for creating/getting the auth token
	TokenSetupURL() string

	// Load loads the provider configuration
	Load() error

	// Save saves the provider configuration
	Save() error

	// BoardIdentifier returns a string identifying the configured board
	BoardIdentifier() string

	// SyncPrompt returns the prompt for sync operations
	SyncPrompt(oneShot bool) string

	// WatchPrompt returns the prompt for watch mode
	WatchPrompt(interval int) string

	// CompletionPromise returns the promise text for sync completion
	CompletionPromise() string

	// Configure runs interactive configuration
	Configure() error

	// ShowConfig displays the current configuration
	ShowConfig() error
}

// ProviderType identifies a board provider.
type ProviderType string

const (
	ProviderNotion ProviderType = "notion"
	ProviderGitHub ProviderType = "github"
)

// ValidProviders returns all valid provider types.
func ValidProviders() []ProviderType {
	return []ProviderType{ProviderNotion, ProviderGitHub}
}

// GetProvider returns a provider by type.
func GetProvider(t ProviderType) Provider {
	switch t {
	case ProviderNotion:
		return NewNotionProvider()
	case ProviderGitHub:
		return NewGitHubProvider()
	default:
		return nil
	}
}

// DefaultProvider returns the default provider type.
func DefaultProvider() ProviderType {
	return ProviderNotion
}
