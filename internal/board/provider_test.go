package board

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zach-source/forge/internal/github"
	"github.com/zach-source/forge/internal/notion"
)

// ============================================================================
// Provider Selection Tests (provider.go)
// ============================================================================

func TestValidProviders(t *testing.T) {
	providers := ValidProviders()

	// Should return exactly 2 providers
	if len(providers) != 2 {
		t.Errorf("ValidProviders() returned %d providers, want 2", len(providers))
	}

	// Should contain both provider types
	found := make(map[ProviderType]bool)
	for _, p := range providers {
		found[p] = true
	}

	if !found[ProviderNotion] {
		t.Error("ValidProviders() missing ProviderNotion")
	}
	if !found[ProviderGitHub] {
		t.Error("ValidProviders() missing ProviderGitHub")
	}
}

func TestGetProvider(t *testing.T) {
	tests := []struct {
		name         string
		providerType ProviderType
		wantName     string
		wantNil      bool
	}{
		{
			name:         "notion provider",
			providerType: ProviderNotion,
			wantName:     "notion",
			wantNil:      false,
		},
		{
			name:         "github provider",
			providerType: ProviderGitHub,
			wantName:     "github",
			wantNil:      false,
		},
		{
			name:         "unknown provider",
			providerType: ProviderType("unknown"),
			wantName:     "",
			wantNil:      true,
		},
		{
			name:         "empty provider type",
			providerType: ProviderType(""),
			wantName:     "",
			wantNil:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetProvider(tt.providerType)

			if tt.wantNil {
				if got != nil {
					t.Errorf("GetProvider(%q) = %v, want nil", tt.providerType, got)
				}
				return
			}

			if got == nil {
				t.Errorf("GetProvider(%q) returned nil, want provider", tt.providerType)
				return
			}

			if got.Name() != tt.wantName {
				t.Errorf("GetProvider(%q).Name() = %q, want %q", tt.providerType, got.Name(), tt.wantName)
			}
		})
	}
}

func TestDefaultProvider(t *testing.T) {
	got := DefaultProvider()
	want := ProviderNotion

	if got != want {
		t.Errorf("DefaultProvider() = %q, want %q", got, want)
	}
}

func TestProviderTypeConstants(t *testing.T) {
	tests := []struct {
		providerType ProviderType
		want         string
	}{
		{ProviderNotion, "notion"},
		{ProviderGitHub, "github"},
	}

	for _, tt := range tests {
		t.Run(string(tt.providerType), func(t *testing.T) {
			if string(tt.providerType) != tt.want {
				t.Errorf("ProviderType constant %q has value %q, want %q",
					tt.providerType, string(tt.providerType), tt.want)
			}
		})
	}
}

// ============================================================================
// Notion Provider Tests (notion_provider.go)
// ============================================================================

func TestNotionProvider_Interface(t *testing.T) {
	p := NewNotionProvider()

	// Verify it implements Provider interface
	var _ Provider = p

	// Test basic info methods
	if p.Name() != "notion" {
		t.Errorf("NotionProvider.Name() = %q, want %q", p.Name(), "notion")
	}

	if p.DisplayName() != "Notion" {
		t.Errorf("NotionProvider.DisplayName() = %q, want %q", p.DisplayName(), "Notion")
	}

	if p.Icon() != "🗂️" {
		t.Errorf("NotionProvider.Icon() = %q, want %q", p.Icon(), "🗂️")
	}
}

func TestNotionProvider_MCPServers(t *testing.T) {
	p := NewNotionProvider()
	servers := p.MCPServers()

	if len(servers) != 1 {
		t.Errorf("NotionProvider.MCPServers() returned %d servers, want 1", len(servers))
	}

	if servers[0] != "notion" {
		t.Errorf("NotionProvider.MCPServers()[0] = %q, want %q", servers[0], "notion")
	}
}

func TestNotionProvider_TokenEnvVar(t *testing.T) {
	p := NewNotionProvider()

	if p.TokenEnvVar() != "NOTION_API_TOKEN" {
		t.Errorf("NotionProvider.TokenEnvVar() = %q, want %q", p.TokenEnvVar(), "NOTION_API_TOKEN")
	}
}

func TestNotionProvider_TokenSetupURL(t *testing.T) {
	p := NewNotionProvider()
	url := p.TokenSetupURL()

	if !strings.HasPrefix(url, "https://") {
		t.Errorf("NotionProvider.TokenSetupURL() = %q, want URL starting with https://", url)
	}

	if !strings.Contains(url, "notion.so") {
		t.Errorf("NotionProvider.TokenSetupURL() = %q, want URL containing notion.so", url)
	}
}

func TestNotionProvider_HasToken(t *testing.T) {
	p := NewNotionProvider()

	// Save original value
	original := os.Getenv("NOTION_API_TOKEN")
	defer os.Setenv("NOTION_API_TOKEN", original)

	// Test without token
	os.Unsetenv("NOTION_API_TOKEN")
	if p.HasToken() {
		t.Error("NotionProvider.HasToken() = true, want false when NOTION_API_TOKEN is not set")
	}

	// Test with empty token
	os.Setenv("NOTION_API_TOKEN", "")
	if p.HasToken() {
		t.Error("NotionProvider.HasToken() = true, want false when NOTION_API_TOKEN is empty")
	}

	// Test with token
	os.Setenv("NOTION_API_TOKEN", "secret_test_token")
	if !p.HasToken() {
		t.Error("NotionProvider.HasToken() = false, want true when NOTION_API_TOKEN is set")
	}
}

func TestNotionProvider_BoardIdentifier_NoConfig(t *testing.T) {
	p := NewNotionProvider()

	// Without config, should return empty string
	if got := p.BoardIdentifier(); got != "" {
		t.Errorf("NotionProvider.BoardIdentifier() = %q, want empty string when no config", got)
	}
}

func TestNotionProvider_SyncPrompt_NoConfig(t *testing.T) {
	p := NewNotionProvider()

	// Without config, should return empty string
	if got := p.SyncPrompt(true); got != "" {
		t.Errorf("NotionProvider.SyncPrompt() = %q, want empty string when no config", got)
	}
}

func TestNotionProvider_WatchPrompt_NoConfig(t *testing.T) {
	p := NewNotionProvider()

	// Without config, should return empty string
	if got := p.WatchPrompt(30); got != "" {
		t.Errorf("NotionProvider.WatchPrompt() = %q, want empty string when no config", got)
	}
}

func TestNotionProvider_CompletionPromise(t *testing.T) {
	p := NewNotionProvider()

	got := p.CompletionPromise()
	if got != "SYNCED" {
		t.Errorf("NotionProvider.CompletionPromise() = %q, want %q", got, "SYNCED")
	}
}

func TestNotionProvider_Save_NoConfig(t *testing.T) {
	p := NewNotionProvider()

	err := p.Save()
	if err == nil {
		t.Error("NotionProvider.Save() should return error when no config")
	}

	if !strings.Contains(err.Error(), "no configuration") {
		t.Errorf("NotionProvider.Save() error = %q, want error about no configuration", err.Error())
	}
}

// ============================================================================
// GitHub Provider Tests (github_provider.go)
// ============================================================================

func TestGitHubProvider_Interface(t *testing.T) {
	p := NewGitHubProvider()

	// Verify it implements Provider interface
	var _ Provider = p

	// Test basic info methods
	if p.Name() != "github" {
		t.Errorf("GitHubProvider.Name() = %q, want %q", p.Name(), "github")
	}

	if p.DisplayName() != "GitHub Projects" {
		t.Errorf("GitHubProvider.DisplayName() = %q, want %q", p.DisplayName(), "GitHub Projects")
	}

	if p.Icon() != "🐙" {
		t.Errorf("GitHubProvider.Icon() = %q, want %q", p.Icon(), "🐙")
	}
}

func TestGitHubProvider_MCPServers(t *testing.T) {
	p := NewGitHubProvider()
	servers := p.MCPServers()

	if len(servers) != 1 {
		t.Errorf("GitHubProvider.MCPServers() returned %d servers, want 1", len(servers))
	}

	if servers[0] != "github" {
		t.Errorf("GitHubProvider.MCPServers()[0] = %q, want %q", servers[0], "github")
	}
}

func TestGitHubProvider_TokenEnvVar(t *testing.T) {
	p := NewGitHubProvider()

	if p.TokenEnvVar() != "GITHUB_TOKEN" {
		t.Errorf("GitHubProvider.TokenEnvVar() = %q, want %q", p.TokenEnvVar(), "GITHUB_TOKEN")
	}
}

func TestGitHubProvider_TokenSetupURL(t *testing.T) {
	p := NewGitHubProvider()
	url := p.TokenSetupURL()

	if !strings.HasPrefix(url, "https://") {
		t.Errorf("GitHubProvider.TokenSetupURL() = %q, want URL starting with https://", url)
	}

	if !strings.Contains(url, "github.com") {
		t.Errorf("GitHubProvider.TokenSetupURL() = %q, want URL containing github.com", url)
	}

	if !strings.Contains(url, "tokens") {
		t.Errorf("GitHubProvider.TokenSetupURL() = %q, want URL containing tokens", url)
	}
}

func TestGitHubProvider_HasToken(t *testing.T) {
	p := NewGitHubProvider()

	// Save original value
	original := os.Getenv("GITHUB_TOKEN")
	defer os.Setenv("GITHUB_TOKEN", original)

	// Test without token
	os.Unsetenv("GITHUB_TOKEN")
	if p.HasToken() {
		t.Error("GitHubProvider.HasToken() = true, want false when GITHUB_TOKEN is not set")
	}

	// Test with empty token
	os.Setenv("GITHUB_TOKEN", "")
	if p.HasToken() {
		t.Error("GitHubProvider.HasToken() = true, want false when GITHUB_TOKEN is empty")
	}

	// Test with token
	os.Setenv("GITHUB_TOKEN", "ghp_test_token")
	if !p.HasToken() {
		t.Error("GitHubProvider.HasToken() = false, want true when GITHUB_TOKEN is set")
	}
}

func TestGitHubProvider_BoardIdentifier_NoConfig(t *testing.T) {
	p := NewGitHubProvider()

	// Without config, should return empty string
	if got := p.BoardIdentifier(); got != "" {
		t.Errorf("GitHubProvider.BoardIdentifier() = %q, want empty string when no config", got)
	}
}

func TestGitHubProvider_SyncPrompt_NoConfig(t *testing.T) {
	p := NewGitHubProvider()

	// Without config, should return empty string
	if got := p.SyncPrompt(true); got != "" {
		t.Errorf("GitHubProvider.SyncPrompt() = %q, want empty string when no config", got)
	}
}

func TestGitHubProvider_WatchPrompt_NoConfig(t *testing.T) {
	p := NewGitHubProvider()

	// Without config, should return empty string
	if got := p.WatchPrompt(30); got != "" {
		t.Errorf("GitHubProvider.WatchPrompt() = %q, want empty string when no config", got)
	}
}

func TestGitHubProvider_CompletionPromise(t *testing.T) {
	p := NewGitHubProvider()

	got := p.CompletionPromise()
	if got != "GITHUB_SYNCED" {
		t.Errorf("GitHubProvider.CompletionPromise() = %q, want %q", got, "GITHUB_SYNCED")
	}
}

func TestGitHubProvider_Save_NoConfig(t *testing.T) {
	p := NewGitHubProvider()

	err := p.Save()
	if err == nil {
		t.Error("GitHubProvider.Save() should return error when no config")
	}

	if !strings.Contains(err.Error(), "no configuration") {
		t.Errorf("GitHubProvider.Save() error = %q, want error about no configuration", err.Error())
	}
}

// ============================================================================
// Prompt Generation Tests (prompt.go)
// ============================================================================

func TestSyncPrompt_OneShot(t *testing.T) {
	databaseID := "test-db-id-12345"
	prompt := SyncPrompt(databaseID, true)

	expectedContents := []string{
		"Forge Board Manager",
		databaseID,
		"Bidirectional Sync",
		"Notion → Beads",
		"Beads → Notion",
		"Status Mapping",
		"bd create",
		"bd list",
		"add_memory",
		"<promise>SYNCED</promise>",
		"one-shot",
	}

	for _, expected := range expectedContents {
		if !strings.Contains(prompt, expected) {
			t.Errorf("SyncPrompt(oneShot=true) missing expected content: %q", expected)
		}
	}
}

func TestSyncPrompt_Interactive(t *testing.T) {
	databaseID := "test-db-id-67890"
	prompt := SyncPrompt(databaseID, false)

	expectedContents := []string{
		"Forge Board Manager",
		databaseID,
		"Interactive Mode",
		"sync",
		"pull",
		"push",
		"status",
	}

	for _, expected := range expectedContents {
		if !strings.Contains(prompt, expected) {
			t.Errorf("SyncPrompt(oneShot=false) missing expected content: %q", expected)
		}
	}

	// Should NOT contain one-shot exit condition
	if strings.Contains(prompt, "one-shot sync") {
		t.Error("SyncPrompt(oneShot=false) should not contain 'one-shot sync'")
	}
}

func TestWatchPrompt(t *testing.T) {
	databaseID := "watch-test-db"
	interval := 60
	prompt := WatchPrompt(databaseID, interval)

	expectedContents := []string{
		"watch mode",
		databaseID,
		"60 seconds",
		"Bidirectional Watch Loop",
		"Sync cycle",
	}

	for _, expected := range expectedContents {
		if !strings.Contains(prompt, expected) {
			t.Errorf("WatchPrompt() missing expected content: %q", expected)
		}
	}
}

func TestCompletionPromise(t *testing.T) {
	got := CompletionPromise()
	want := "SYNCED"

	if got != want {
		t.Errorf("CompletionPromise() = %q, want %q", got, want)
	}
}

// ============================================================================
// GitHub Prompt Generation Tests (github.go)
// ============================================================================

func TestGitHubSyncPrompt_OneShot(t *testing.T) {
	owner := "testowner"
	projectNumber := 42
	repo := "testrepo"
	prompt := GitHubSyncPrompt(owner, projectNumber, repo, true)

	expectedContents := []string{
		"Forge Board Manager",
		"GitHub Projects",
		owner,
		"42",
		repo,
		"gh project item-list",
		"gh project item-create",
		"bd list",
		"bd create",
		"<promise>GITHUB_SYNCED</promise>",
		"one-shot",
	}

	for _, expected := range expectedContents {
		if !strings.Contains(prompt, expected) {
			t.Errorf("GitHubSyncPrompt(oneShot=true) missing expected content: %q", expected)
		}
	}
}

func TestGitHubSyncPrompt_Interactive(t *testing.T) {
	owner := "interactiveowner"
	projectNumber := 99
	repo := ""
	prompt := GitHubSyncPrompt(owner, projectNumber, repo, false)

	expectedContents := []string{
		owner,
		"99",
		"Interactive Mode",
		"sync",
		"pull",
		"push",
	}

	for _, expected := range expectedContents {
		if !strings.Contains(prompt, expected) {
			t.Errorf("GitHubSyncPrompt(oneShot=false) missing expected content: %q", expected)
		}
	}
}

func TestGitHubSyncPrompt_NoRepo(t *testing.T) {
	owner := "norepoowner"
	projectNumber := 1
	prompt := GitHubSyncPrompt(owner, projectNumber, "", true)

	// Should contain owner and project number
	if !strings.Contains(prompt, owner) {
		t.Errorf("GitHubSyncPrompt() missing owner: %q", owner)
	}

	// Verify prompt is valid even without repo
	if prompt == "" {
		t.Error("GitHubSyncPrompt() returned empty string")
	}
}

func TestGitHubWatchPrompt(t *testing.T) {
	owner := "watchowner"
	projectNumber := 10
	repo := "watchrepo"
	interval := 30
	prompt := GitHubWatchPrompt(owner, projectNumber, repo, interval)

	expectedContents := []string{
		"watch mode",
		owner,
		"10",
		"30 seconds",
		"Watch Loop",
	}

	for _, expected := range expectedContents {
		if !strings.Contains(prompt, expected) {
			t.Errorf("GitHubWatchPrompt() missing expected content: %q", expected)
		}
	}
}

func TestGitHubCompletionPromise(t *testing.T) {
	got := GitHubCompletionPromise()
	want := "GITHUB_SYNCED"

	if got != want {
		t.Errorf("GitHubCompletionPromise() = %q, want %q", got, want)
	}
}

func TestGitHubWorkerAssignPrompt(t *testing.T) {
	owner := "assignowner"
	repo := "assignrepo"
	issueNumber := 123
	workerName := "alpha"
	prompt := GitHubWorkerAssignPrompt(owner, repo, issueNumber, workerName)

	expectedContents := []string{
		owner,
		repo,
		"123",
		"alpha",
		"add_issue_comment",
		"update_issue",
		"add_memory",
		"forge-worker-assignments",
	}

	for _, expected := range expectedContents {
		if !strings.Contains(prompt, expected) {
			t.Errorf("GitHubWorkerAssignPrompt() missing expected content: %q", expected)
		}
	}
}

// ============================================================================
// Provider Comparison Tests
// ============================================================================

func TestAllProvidersHaveUniqueNames(t *testing.T) {
	providers := ValidProviders()
	names := make(map[string]bool)

	for _, pt := range providers {
		p := GetProvider(pt)
		if p == nil {
			t.Errorf("GetProvider(%q) returned nil", pt)
			continue
		}

		name := p.Name()
		if names[name] {
			t.Errorf("Duplicate provider name: %q", name)
		}
		names[name] = true
	}
}

func TestAllProvidersHaveUniqueDisplayNames(t *testing.T) {
	providers := ValidProviders()
	displayNames := make(map[string]bool)

	for _, pt := range providers {
		p := GetProvider(pt)
		if p == nil {
			continue
		}

		displayName := p.DisplayName()
		if displayNames[displayName] {
			t.Errorf("Duplicate provider display name: %q", displayName)
		}
		displayNames[displayName] = true
	}
}

func TestAllProvidersHaveUniqueTokenEnvVars(t *testing.T) {
	providers := ValidProviders()
	envVars := make(map[string]bool)

	for _, pt := range providers {
		p := GetProvider(pt)
		if p == nil {
			continue
		}

		envVar := p.TokenEnvVar()
		if envVars[envVar] {
			t.Errorf("Duplicate token env var: %q", envVar)
		}
		envVars[envVar] = true
	}
}

func TestAllProvidersHaveValidTokenSetupURLs(t *testing.T) {
	providers := ValidProviders()

	for _, pt := range providers {
		p := GetProvider(pt)
		if p == nil {
			continue
		}

		url := p.TokenSetupURL()
		if url == "" {
			t.Errorf("Provider %q has empty TokenSetupURL", p.Name())
		}
		if !strings.HasPrefix(url, "https://") {
			t.Errorf("Provider %q TokenSetupURL %q doesn't start with https://", p.Name(), url)
		}
	}
}

func TestAllProvidersHaveNonEmptyMCPServers(t *testing.T) {
	providers := ValidProviders()

	for _, pt := range providers {
		p := GetProvider(pt)
		if p == nil {
			continue
		}

		servers := p.MCPServers()
		if len(servers) == 0 {
			t.Errorf("Provider %q has empty MCPServers", p.Name())
		}
	}
}

func TestAllProvidersHaveNonEmptyIcons(t *testing.T) {
	providers := ValidProviders()

	for _, pt := range providers {
		p := GetProvider(pt)
		if p == nil {
			continue
		}

		icon := p.Icon()
		if icon == "" {
			t.Errorf("Provider %q has empty Icon", p.Name())
		}
	}
}

func TestAllProvidersHaveCompletionPromises(t *testing.T) {
	providers := ValidProviders()

	for _, pt := range providers {
		p := GetProvider(pt)
		if p == nil {
			continue
		}

		promise := p.CompletionPromise()
		if promise == "" {
			t.Errorf("Provider %q has empty CompletionPromise", p.Name())
		}
	}
}

// ============================================================================
// Edge Cases and Error Handling
// ============================================================================

func TestSyncPrompt_EmptyDatabaseID(t *testing.T) {
	prompt := SyncPrompt("", true)

	// Should still generate a prompt (even if database ID is empty)
	if prompt == "" {
		t.Error("SyncPrompt() returned empty string for empty database ID")
	}
}

func TestGitHubSyncPrompt_ZeroProjectNumber(t *testing.T) {
	prompt := GitHubSyncPrompt("owner", 0, "repo", true)

	// Should still generate a prompt
	if prompt == "" {
		t.Error("GitHubSyncPrompt() returned empty string for zero project number")
	}

	// Should contain the zero
	if !strings.Contains(prompt, "0") {
		t.Error("GitHubSyncPrompt() should contain project number 0")
	}
}

func TestWatchPrompt_ZeroInterval(t *testing.T) {
	prompt := WatchPrompt("db-id", 0)

	// Should still generate a prompt
	if prompt == "" {
		t.Error("WatchPrompt() returned empty string for zero interval")
	}
}

func TestGitHubWatchPrompt_ZeroInterval(t *testing.T) {
	prompt := GitHubWatchPrompt("owner", 1, "repo", 0)

	// Should still generate a prompt
	if prompt == "" {
		t.Error("GitHubWatchPrompt() returned empty string for zero interval")
	}
}

// ============================================================================
// Configured Provider Tests (with config set)
// ============================================================================

func TestNotionProvider_WithConfig(t *testing.T) {
	p := NewNotionProvider()

	// Set config directly (using internal field access from same package)
	p.config = &notion.Config{
		DatabaseID: "test-database-id-123",
		BoardID:    "test-board-id",
	}

	// Test BoardIdentifier with config
	if got := p.BoardIdentifier(); got != "test-database-id-123" {
		t.Errorf("NotionProvider.BoardIdentifier() = %q, want %q", got, "test-database-id-123")
	}

	// Test SyncPrompt with config (oneShot=true)
	syncPrompt := p.SyncPrompt(true)
	if syncPrompt == "" {
		t.Error("NotionProvider.SyncPrompt() returned empty string with config set")
	}
	if !strings.Contains(syncPrompt, "test-database-id-123") {
		t.Error("NotionProvider.SyncPrompt() should contain database ID")
	}

	// Test SyncPrompt with config (oneShot=false)
	interactivePrompt := p.SyncPrompt(false)
	if interactivePrompt == "" {
		t.Error("NotionProvider.SyncPrompt(false) returned empty string with config set")
	}

	// Test WatchPrompt with config
	watchPrompt := p.WatchPrompt(60)
	if watchPrompt == "" {
		t.Error("NotionProvider.WatchPrompt() returned empty string with config set")
	}
	if !strings.Contains(watchPrompt, "test-database-id-123") {
		t.Error("NotionProvider.WatchPrompt() should contain database ID")
	}
}

func TestGitHubProvider_WithConfig(t *testing.T) {
	p := NewGitHubProvider()

	// Set config directly
	p.config = &github.Config{
		Owner:         "test-owner",
		Repo:          "test-repo",
		ProjectNumber: 42,
	}

	// Test BoardIdentifier with config
	boardID := p.BoardIdentifier()
	if boardID == "" {
		t.Error("GitHubProvider.BoardIdentifier() returned empty string with config set")
	}
	if !strings.Contains(boardID, "test-owner") {
		t.Error("GitHubProvider.BoardIdentifier() should contain owner")
	}
	if !strings.Contains(boardID, "42") {
		t.Error("GitHubProvider.BoardIdentifier() should contain project number")
	}

	// Test SyncPrompt with config (oneShot=true)
	syncPrompt := p.SyncPrompt(true)
	if syncPrompt == "" {
		t.Error("GitHubProvider.SyncPrompt() returned empty string with config set")
	}
	if !strings.Contains(syncPrompt, "test-owner") {
		t.Error("GitHubProvider.SyncPrompt() should contain owner")
	}

	// Test SyncPrompt with config (oneShot=false)
	interactivePrompt := p.SyncPrompt(false)
	if interactivePrompt == "" {
		t.Error("GitHubProvider.SyncPrompt(false) returned empty string with config set")
	}

	// Test WatchPrompt with config
	watchPrompt := p.WatchPrompt(30)
	if watchPrompt == "" {
		t.Error("GitHubProvider.WatchPrompt() returned empty string with config set")
	}
	if !strings.Contains(watchPrompt, "test-owner") {
		t.Error("GitHubProvider.WatchPrompt() should contain owner")
	}
}

func TestGitHubProvider_WithConfig_NoRepo(t *testing.T) {
	p := NewGitHubProvider()

	// Set config without repo (user/org project)
	p.config = &github.Config{
		Owner:         "some-user",
		Repo:          "", // No repo
		ProjectNumber: 10,
	}

	// Test BoardIdentifier with no repo
	boardID := p.BoardIdentifier()
	if !strings.Contains(boardID, "some-user") {
		t.Error("GitHubProvider.BoardIdentifier() should contain owner")
	}
	if !strings.Contains(boardID, "10") {
		t.Error("GitHubProvider.BoardIdentifier() should contain project number")
	}
}

// ============================================================================
// Provider Load/Save Tests with Temp Files
// ============================================================================

func TestNotionProvider_Load_NoConfig(t *testing.T) {
	p := NewNotionProvider()

	// Save original HOME
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	// Use temp dir as HOME
	tempDir := t.TempDir()
	os.Setenv("HOME", tempDir)

	// Load should fail when no config file exists
	err := p.Load()
	if err == nil {
		t.Error("NotionProvider.Load() should return error when no config file exists")
	}
}

func TestGitHubProvider_Load_NoConfig(t *testing.T) {
	p := NewGitHubProvider()

	// Save original HOME
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	// Use temp dir as HOME
	tempDir := t.TempDir()
	os.Setenv("HOME", tempDir)

	// Load should fail when no config file exists
	err := p.Load()
	if err == nil {
		t.Error("GitHubProvider.Load() should return error when no config file exists")
	}
}

func TestNotionProvider_SaveAndLoad(t *testing.T) {
	// Save original HOME
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	// Use temp dir as HOME
	tempDir := t.TempDir()
	os.Setenv("HOME", tempDir)

	// Create .forge directory
	forgeDir := filepath.Join(tempDir, ".forge")
	if err := os.MkdirAll(forgeDir, 0755); err != nil {
		t.Fatalf("Failed to create .forge dir: %v", err)
	}

	// Create a provider with config
	p := NewNotionProvider()
	p.config = &notion.Config{
		DatabaseID: "save-test-db-id",
	}

	// Save should succeed
	err := p.Save()
	if err != nil {
		t.Errorf("NotionProvider.Save() error = %v", err)
	}

	// Create new provider and load
	p2 := NewNotionProvider()
	err = p2.Load()
	if err != nil {
		t.Errorf("NotionProvider.Load() error = %v", err)
	}

	if p2.BoardIdentifier() != "save-test-db-id" {
		t.Errorf("NotionProvider.Load() BoardIdentifier = %q, want %q",
			p2.BoardIdentifier(), "save-test-db-id")
	}
}

func TestGitHubProvider_SaveAndLoad(t *testing.T) {
	// Save original HOME
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	// Use temp dir as HOME
	tempDir := t.TempDir()
	os.Setenv("HOME", tempDir)

	// Create .forge directory
	forgeDir := filepath.Join(tempDir, ".forge")
	if err := os.MkdirAll(forgeDir, 0755); err != nil {
		t.Fatalf("Failed to create .forge dir: %v", err)
	}

	// Create a provider with config
	p := NewGitHubProvider()
	p.config = &github.Config{
		Owner:         "save-test-owner",
		Repo:          "save-test-repo",
		ProjectNumber: 99,
	}

	// Save should succeed
	err := p.Save()
	if err != nil {
		t.Errorf("GitHubProvider.Save() error = %v", err)
	}

	// Create new provider and load
	p2 := NewGitHubProvider()
	err = p2.Load()
	if err != nil {
		t.Errorf("GitHubProvider.Load() error = %v", err)
	}

	boardID := p2.BoardIdentifier()
	if !strings.Contains(boardID, "save-test-owner") {
		t.Errorf("GitHubProvider.Load() BoardIdentifier = %q, should contain owner", boardID)
	}
}
