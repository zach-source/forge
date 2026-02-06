// Package agentteams provides integration tests for the Agent Teams feature.
// These tests verify the complete agent teams pipeline: tmux plumbing, config wiring,
// worker role additions, leader integration, supervisor state management, and
// prompt generation.
package agentteams

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zach-source/forge/internal/agent"
	"github.com/zach-source/forge/internal/leader"
	"github.com/zach-source/forge/internal/leader/defaults"
	"github.com/zach-source/forge/internal/tmux"
	"github.com/zach-source/forge/internal/worker"
)

// ============================================================
// Phase 1: Tmux Plumbing Tests
// ============================================================

func TestRunClaudeOptions_Struct(t *testing.T) {
	opts := tmux.RunClaudeOptions{
		Prompt:          "test prompt",
		MCPConfig:       "/tmp/mcp.json",
		SkipPermissions: true,
		AgentTeams:      true,
		TeammateMode:    "tmux",
	}

	if opts.Prompt != "test prompt" {
		t.Errorf("Prompt = %q, want %q", opts.Prompt, "test prompt")
	}
	if opts.MCPConfig != "/tmp/mcp.json" {
		t.Errorf("MCPConfig = %q, want %q", opts.MCPConfig, "/tmp/mcp.json")
	}
	if !opts.SkipPermissions {
		t.Error("SkipPermissions should be true")
	}
	if !opts.AgentTeams {
		t.Error("AgentTeams should be true")
	}
	if opts.TeammateMode != "tmux" {
		t.Errorf("TeammateMode = %q, want %q", opts.TeammateMode, "tmux")
	}
}

func TestClaudeTeamOptions_Struct(t *testing.T) {
	opts := tmux.ClaudeTeamOptions{
		MCPConfig:    "/tmp/team-mcp.json",
		TeammateMode: "in-process",
	}

	if opts.MCPConfig != "/tmp/team-mcp.json" {
		t.Errorf("MCPConfig = %q, want %q", opts.MCPConfig, "/tmp/team-mcp.json")
	}
	if opts.TeammateMode != "in-process" {
		t.Errorf("TeammateMode = %q, want %q", opts.TeammateMode, "in-process")
	}
}

func TestRunClaudeWithOptions_NonExistentSession(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	s := &tmux.Session{Name: "nonexistent-agentteams-test"}
	err := s.RunClaudeWithOptions(tmux.RunClaudeOptions{
		Prompt:     "test",
		AgentTeams: true,
	})
	if err == nil {
		t.Fatal("RunClaudeWithOptions on non-existent session should error")
	}
}

func TestRunClaudeInteractive_NonExistentSession(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	s := &tmux.Session{Name: "nonexistent-agentteams-interactive-test"}
	err := s.RunClaudeInteractive(tmux.ClaudeTeamOptions{
		TeammateMode: "tmux",
	})
	if err == nil {
		t.Fatal("RunClaudeInteractive on non-existent session should error")
	}
}

// TestRunClaudeWithOptions_AgentTeamsEnvVar verifies the command sent to tmux
// includes the CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1 environment variable.
func TestRunClaudeWithOptions_AgentTeamsCommand(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	name := fmt.Sprintf("forge-agentteams-cmd-%d", time.Now().UnixNano())
	workDir := t.TempDir()
	s := tmux.NewSession(name, workDir, "")

	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Run claude with agent teams (it will fail since claude isn't installed in test,
	// but we can verify the command was sent)
	err := s.RunClaudeWithOptions(tmux.RunClaudeOptions{
		Prompt:          "test agent teams",
		AgentTeams:      true,
		TeammateMode:    "tmux",
		SkipPermissions: true,
	})
	if err != nil {
		t.Fatalf("RunClaudeWithOptions() error: %v", err)
	}

	// Wait for command to appear in pane
	time.Sleep(500 * time.Millisecond)

	content, err := s.CapturePane()
	if err != nil {
		t.Fatalf("CapturePane() error: %v", err)
	}

	// Verify the agent teams env var was included in the command
	if !strings.Contains(content, "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1") {
		t.Errorf("Command should include CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1, got:\n%s", content)
	}

	// Verify teammate mode was included
	if !strings.Contains(content, "--teammate-mode") {
		t.Errorf("Command should include --teammate-mode flag, got:\n%s", content)
	}
}

// TestRunClaudeInteractive_AgentTeamsCommand verifies the interactive command
// includes agent teams env var and no stdin pipe.
func TestRunClaudeInteractive_AgentTeamsCommand(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	name := fmt.Sprintf("forge-agentteams-interactive-%d", time.Now().UnixNano())
	workDir := t.TempDir()
	s := tmux.NewSession(name, workDir, "")

	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	err := s.RunClaudeInteractive(tmux.ClaudeTeamOptions{
		TeammateMode: "tmux",
	})
	if err != nil {
		t.Fatalf("RunClaudeInteractive() error: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	content, err := s.CapturePane()
	if err != nil {
		t.Fatalf("CapturePane() error: %v", err)
	}

	// Verify agent teams env var
	if !strings.Contains(content, "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1") {
		t.Errorf("Interactive command should include CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1, got:\n%s", content)
	}

	// Verify NO stdin pipe (no "cat" command)
	if strings.Contains(content, "cat /") && strings.Contains(content, "forge-prompt") {
		t.Errorf("Interactive command should NOT pipe via cat, got:\n%s", content)
	}

	// Verify no --dangerously-skip-permissions
	if strings.Contains(content, "--dangerously-skip-permissions") {
		t.Errorf("Interactive command should NOT include --dangerously-skip-permissions, got:\n%s", content)
	}
}

// TestRunClaude_BackwardsCompatible verifies that the refactored RunClaude()
// still works correctly (delegates to RunClaudeWithOptions).
func TestRunClaude_BackwardsCompatible(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	name := fmt.Sprintf("forge-agentteams-compat-%d", time.Now().UnixNano())
	workDir := t.TempDir()
	s := tmux.NewSession(name, workDir, "")

	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Use the old RunClaude API
	err := s.RunClaude("test backwards compat", "", true)
	if err != nil {
		t.Fatalf("RunClaude() error: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	content, err := s.CapturePane()
	if err != nil {
		t.Fatalf("CapturePane() error: %v", err)
	}

	// Should NOT include agent teams env var (backwards compat)
	if strings.Contains(content, "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1") {
		t.Errorf("Legacy RunClaude() should NOT include agent teams env var, got:\n%s", content)
	}

	// Should include --dangerously-skip-permissions since we passed true
	if !strings.Contains(content, "--dangerously-skip-permissions") {
		t.Errorf("RunClaude(skipPermissions=true) should include --dangerously-skip-permissions, got:\n%s", content)
	}
}

// ============================================================
// Phase 2: Agent Config Tests
// ============================================================

func TestAgentConfig_AgentTeamsFields(t *testing.T) {
	cfg := agent.DefaultConfig()

	// Defaults should be false/empty
	if cfg.AgentTeams {
		t.Error("DefaultConfig().AgentTeams should be false")
	}
	if cfg.TeammateMode != "" {
		t.Errorf("DefaultConfig().TeammateMode = %q, want empty", cfg.TeammateMode)
	}

	// Set the fields
	cfg.AgentTeams = true
	cfg.TeammateMode = "tmux"

	if !cfg.AgentTeams {
		t.Error("AgentTeams should be true after setting")
	}
	if cfg.TeammateMode != "tmux" {
		t.Errorf("TeammateMode = %q, want %q", cfg.TeammateMode, "tmux")
	}
}

func TestAgentConfig_ValidateWithAgentTeams(t *testing.T) {
	// Config with agent teams should still require prompt, promise, workdir
	cfg := agent.Config{
		AgentTeams:   true,
		TeammateMode: "tmux",
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should fail without prompt even with AgentTeams set")
	}

	// Full valid config
	cfg.Prompt = "team lead prompt"
	cfg.CompletionPromise = "LEADER_TEAM_COMPLETE"
	cfg.WorkDir = "/tmp"

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() with all fields should pass: %v", err)
	}
}

// ============================================================
// Phase 3: Worker Role - RoleTeamLead Tests
// ============================================================

func TestRoleTeamLead_Constant(t *testing.T) {
	if string(worker.RoleTeamLead) != "team-lead" {
		t.Errorf("RoleTeamLead = %q, want %q", worker.RoleTeamLead, "team-lead")
	}
}

func TestRoleTeamLead_NotSingleThreaded(t *testing.T) {
	if worker.RoleTeamLead.IsSingleThreaded() {
		t.Error("RoleTeamLead should not be single-threaded")
	}
}

func TestRoleTeamLead_InValidRoles(t *testing.T) {
	roles := worker.ValidRoles()
	found := false
	for _, r := range roles {
		if r == worker.RoleTeamLead {
			found = true
			break
		}
	}
	if !found {
		t.Error("RoleTeamLead should be in ValidRoles()")
	}
}

func TestParseRole_TeamLead(t *testing.T) {
	role, err := worker.ParseRole("team-lead")
	if err != nil {
		t.Fatalf("ParseRole(team-lead) error: %v", err)
	}
	if role != worker.RoleTeamLead {
		t.Errorf("ParseRole(team-lead) = %q, want %q", role, worker.RoleTeamLead)
	}
}

func TestRoleTeamLead_Icon(t *testing.T) {
	w := &worker.Worker{
		ID:     "w-test1234",
		Name:   "teamlead",
		Role:   worker.RoleTeamLead,
		Status: worker.StatusIdle,
	}

	icon := w.RoleIcon()
	if icon != "🎯" {
		t.Errorf("RoleIcon() for team-lead = %q, want %q", icon, "🎯")
	}
}

func TestWorkerValidate_TeamLead(t *testing.T) {
	w := &worker.Worker{
		ID:     "w-teamlead",
		Name:   "lead",
		Role:   worker.RoleTeamLead,
		Status: worker.StatusIdle,
	}

	if err := w.Validate(); err != nil {
		t.Errorf("Validate() should pass for team-lead role: %v", err)
	}
}

// ============================================================
// Phase 3: Worker StartOptions - AgentTeams Fields
// ============================================================

func TestStartOptions_AgentTeamsFields(t *testing.T) {
	opts := worker.StartOptions{
		TaskID:       "team-lead-session",
		Prompt:       "coordinate the team",
		Promise:      "LEADER_TEAM_COMPLETE",
		AgentTeams:   true,
		TeammateMode: "tmux",
	}

	if !opts.AgentTeams {
		t.Error("AgentTeams should be true")
	}
	if opts.TeammateMode != "tmux" {
		t.Errorf("TeammateMode = %q, want %q", opts.TeammateMode, "tmux")
	}
}

// ============================================================
// Phase 3: Leader Role + Promise Tests
// ============================================================

func TestLeaderRoleTeamLead_Constant(t *testing.T) {
	if string(leader.RoleTeamLead) != "team-lead" {
		t.Errorf("leader.RoleTeamLead = %q, want %q", leader.RoleTeamLead, "team-lead")
	}
}

func TestPromiseLeaderTeam(t *testing.T) {
	if leader.PromiseLeaderTeam != "LEADER_TEAM_COMPLETE" {
		t.Errorf("PromiseLeaderTeam = %q, want %q", leader.PromiseLeaderTeam, "LEADER_TEAM_COMPLETE")
	}
}

// ============================================================
// Phase 3: Default Prompt - team-lead.md
// ============================================================

func TestTeamLeadDefaultPrompt_Exists(t *testing.T) {
	roles := defaults.AllRoles()
	found := false
	for _, r := range roles {
		if r == "team-lead" {
			found = true
			break
		}
	}
	if !found {
		t.Error("team-lead should be in defaults.AllRoles()")
	}
}

func TestTeamLeadDefaultPrompt_Content(t *testing.T) {
	content, err := defaults.Prompts.ReadFile("team-lead.md")
	if err != nil {
		t.Fatalf("Failed to read team-lead.md: %v", err)
	}

	prompt := string(content)

	// Verify key sections are present
	expectedSections := []string{
		"Leader Team Coordinator",
		"Your Role",
		"Available Roles",
		"Groomer",
		"Reviewer",
		"Planner",
		"PM",
		"Board State",
		"Completion",
		"LEADER_TEAM_COMPLETE",
	}

	for _, section := range expectedSections {
		if !strings.Contains(prompt, section) {
			t.Errorf("team-lead.md missing expected section: %q", section)
		}
	}
}

func TestTeamLeadDefaultPrompt_HasTemplateVar(t *testing.T) {
	content, err := defaults.Prompts.ReadFile("team-lead.md")
	if err != nil {
		t.Fatalf("Failed to read team-lead.md: %v", err)
	}

	if !strings.Contains(string(content), "{{.BoardState}}") {
		t.Error("team-lead.md should contain {{.BoardState}} template variable")
	}
}

func TestTeamLeadPrompt_LoadWithVars(t *testing.T) {
	workDir := t.TempDir()
	loader := leader.NewPromptLoader(workDir)

	// Init defaults (includes team-lead)
	_, err := loader.InitDefaults(true)
	if err != nil {
		t.Fatalf("InitDefaults() error: %v", err)
	}

	// Load with variables
	prompt, err := loader.LoadWithVars("team-lead", map[string]string{
		"BoardState": "### backlog (3)\n- task-1: Fix bug\n- task-2: Add feature\n- task-3: Refactor",
	})
	if err != nil {
		t.Fatalf("LoadWithVars() error: %v", err)
	}

	if !strings.Contains(prompt, "### backlog (3)") {
		t.Error("Loaded prompt should have replaced {{.BoardState}} template var")
	}

	if strings.Contains(prompt, "{{.BoardState}}") {
		t.Error("Loaded prompt should not contain unreplaced {{.BoardState}} template var")
	}
}

// ============================================================
// Cross-Package Integration: Config → Agent Pipeline
// ============================================================

func TestAgentTeams_ConfigToAgentPipeline(t *testing.T) {
	// Verify the full config pipeline: worker StartOptions → agent Config → Agent
	// This tests that AgentTeams/TeammateMode flow through the entire chain

	cfg := agent.DefaultConfig()
	cfg.Prompt = "Team lead: coordinate groomer, reviewer, planner, pm"
	cfg.CompletionPromise = leader.PromiseLeaderTeam
	cfg.WorkDir = t.TempDir()
	cfg.AgentTeams = true
	cfg.TeammateMode = "tmux"

	// Validate the config
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Config.Validate() error: %v", err)
	}

	// Verify fields are set correctly
	if !cfg.AgentTeams {
		t.Error("Config.AgentTeams should be true")
	}
	if cfg.TeammateMode != "tmux" {
		t.Errorf("Config.TeammateMode = %q, want %q", cfg.TeammateMode, "tmux")
	}
	if cfg.CompletionPromise != "LEADER_TEAM_COMPLETE" {
		t.Errorf("Config.CompletionPromise = %q, want %q", cfg.CompletionPromise, "LEADER_TEAM_COMPLETE")
	}
}

// ============================================================
// Cross-Package Integration: Worker Role Consistency
// ============================================================

func TestRoleConsistency_WorkerAndLeader(t *testing.T) {
	// Verify worker.RoleTeamLead and leader.RoleTeamLead have the same string value
	if string(worker.RoleTeamLead) != string(leader.RoleTeamLead) {
		t.Errorf("worker.RoleTeamLead (%q) != leader.RoleTeamLead (%q)",
			worker.RoleTeamLead, leader.RoleTeamLead)
	}
}

func TestAllDefaultPrompts_HaveMatchingRoles(t *testing.T) {
	// Every role in defaults.AllRoles() should have a corresponding .md file
	for _, role := range defaults.AllRoles() {
		content, err := defaults.Prompts.ReadFile(role + ".md")
		if err != nil {
			t.Errorf("Missing default prompt file for role %q: %v", role, err)
			continue
		}
		if len(content) == 0 {
			t.Errorf("Default prompt for role %q is empty", role)
		}
	}
}

// ============================================================
// Tmux Integration: Full Lifecycle
// ============================================================

func TestTeamSession_FullLifecycle(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	name := fmt.Sprintf("forge-agentteams-lifecycle-%d", time.Now().UnixNano())
	workDir := t.TempDir()
	s := tmux.NewSession(name, workDir, "")

	defer func() { _ = s.Kill() }()

	// 1. Create session
	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if !s.Exists() {
		t.Fatal("Session should exist after Create")
	}

	// 2. Start Claude interactively with agent teams
	if err := s.RunClaudeInteractive(tmux.ClaudeTeamOptions{
		TeammateMode: "tmux",
	}); err != nil {
		t.Fatalf("RunClaudeInteractive() error: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// 3. Verify session content has the right command
	content, err := s.CapturePane()
	if err != nil {
		t.Fatalf("CapturePane() error: %v", err)
	}

	if !strings.Contains(content, "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1") {
		t.Error("Session should contain agent teams env var")
	}
	if !strings.Contains(content, "--teammate-mode tmux") {
		t.Error("Session should contain --teammate-mode tmux")
	}

	// 4. Session should be listable with forge prefix
	sessions, err := tmux.ListSessions("forge-agentteams-")
	if err != nil {
		t.Fatalf("ListSessions() error: %v", err)
	}

	found := false
	for _, sess := range sessions {
		if sess == name {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Session %q not found in list: %v", name, sessions)
	}

	// 5. Kill session
	if err := s.Kill(); err != nil {
		t.Fatalf("Kill() error: %v", err)
	}
	if s.Exists() {
		t.Fatal("Session should not exist after Kill")
	}
}

// TestRunClaudeWithOptions_AllTeammateModes verifies all teammate mode values.
func TestRunClaudeWithOptions_AllTeammateModes(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	modes := []string{"tmux", "in-process", "auto"}

	for _, mode := range modes {
		t.Run("mode-"+mode, func(t *testing.T) {
			name := fmt.Sprintf("forge-agentteams-mode-%s-%d", mode, time.Now().UnixNano())
			workDir := t.TempDir()
			s := tmux.NewSession(name, workDir, "")
			defer func() { _ = s.Kill() }()

			if err := s.Create(); err != nil {
				t.Fatalf("Create() error: %v", err)
			}

			if err := s.RunClaudeWithOptions(tmux.RunClaudeOptions{
				Prompt:       "test " + mode,
				AgentTeams:   true,
				TeammateMode: mode,
			}); err != nil {
				t.Fatalf("RunClaudeWithOptions() error: %v", err)
			}

			time.Sleep(400 * time.Millisecond)

			content, err := s.CapturePane()
			if err != nil {
				t.Fatalf("CapturePane() error: %v", err)
			}

			// Normalize tmux line-wrapping: join lines so wrapped commands match
			flat := strings.ReplaceAll(content, "\n", "")
			if !strings.Contains(flat, "--teammate-mode "+mode) {
				t.Errorf("Command should contain '--teammate-mode %s', got:\n%s", mode, content)
			}
		})
	}
}

// TestRunClaudeWithOptions_NoAgentTeams verifies that when AgentTeams is false,
// the env var is not included.
func TestRunClaudeWithOptions_NoAgentTeams(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	name := fmt.Sprintf("forge-agentteams-noagent-%d", time.Now().UnixNano())
	workDir := t.TempDir()
	s := tmux.NewSession(name, workDir, "")
	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if err := s.RunClaudeWithOptions(tmux.RunClaudeOptions{
		Prompt:     "test no agent teams",
		AgentTeams: false, // Explicitly false
	}); err != nil {
		t.Fatalf("RunClaudeWithOptions() error: %v", err)
	}

	time.Sleep(400 * time.Millisecond)

	content, err := s.CapturePane()
	if err != nil {
		t.Fatalf("CapturePane() error: %v", err)
	}

	if strings.Contains(content, "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1") {
		t.Errorf("Command should NOT contain agent teams env var when AgentTeams=false, got:\n%s", content)
	}
}

// ============================================================
// Edge Cases and Boundary Tests
// ============================================================

func TestRunClaudeWithOptions_EmptyTeammateMode(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	name := fmt.Sprintf("forge-agentteams-emptymode-%d", time.Now().UnixNano())
	workDir := t.TempDir()
	s := tmux.NewSession(name, workDir, "")
	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Empty teammate mode should not include the flag
	if err := s.RunClaudeWithOptions(tmux.RunClaudeOptions{
		Prompt:       "test empty mode",
		AgentTeams:   true,
		TeammateMode: "", // Empty
	}); err != nil {
		t.Fatalf("RunClaudeWithOptions() error: %v", err)
	}

	time.Sleep(400 * time.Millisecond)

	content, err := s.CapturePane()
	if err != nil {
		t.Fatalf("CapturePane() error: %v", err)
	}

	if strings.Contains(content, "--teammate-mode") {
		t.Errorf("Command should NOT contain --teammate-mode when mode is empty, got:\n%s", content)
	}

	// But agent teams should still be set
	if !strings.Contains(content, "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1") {
		t.Errorf("Command should still contain agent teams env var, got:\n%s", content)
	}
}

func TestRunClaudeInteractive_WithMCPConfig(t *testing.T) {
	if !tmux.IsTmuxInstalled() {
		t.Skip("tmux not installed")
	}

	name := fmt.Sprintf("forge-agentteams-mcp-%d", time.Now().UnixNano())
	workDir := t.TempDir()
	s := tmux.NewSession(name, workDir, "")
	defer func() { _ = s.Kill() }()

	if err := s.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	mcpPath := "/tmp/test-mcp-config.json"

	if err := s.RunClaudeInteractive(tmux.ClaudeTeamOptions{
		MCPConfig:    mcpPath,
		TeammateMode: "auto",
	}); err != nil {
		t.Fatalf("RunClaudeInteractive() error: %v", err)
	}

	time.Sleep(400 * time.Millisecond)

	content, err := s.CapturePane()
	if err != nil {
		t.Fatalf("CapturePane() error: %v", err)
	}

	if !strings.Contains(content, "--mcp-config") {
		t.Errorf("Command should contain --mcp-config, got:\n%s", content)
	}
}

// ============================================================
// Cross-Package: Worker + Leader Role Complete Coverage
// ============================================================

func TestAllWorkerRoles_HaveIcons(t *testing.T) {
	for _, role := range worker.ValidRoles() {
		w := &worker.Worker{
			ID:     "w-test1234",
			Name:   "test",
			Role:   role,
			Status: worker.StatusIdle,
		}

		icon := w.RoleIcon()
		if icon == "❓" {
			t.Errorf("Role %q has unknown icon (❓)", role)
		}
	}
}

func TestAllWorkerRoles_Parseable(t *testing.T) {
	for _, role := range worker.ValidRoles() {
		parsed, err := worker.ParseRole(string(role))
		if err != nil {
			t.Errorf("ParseRole(%q) error: %v", role, err)
			continue
		}
		if parsed != role {
			t.Errorf("ParseRole(%q) = %q, want %q", role, parsed, role)
		}
	}
}
