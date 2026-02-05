package leader

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	tests := []struct {
		role            Role
		wantMaxIter     int
		wantSkipPerms   bool
		wantBranch      string
		wantEnvironment string
	}{
		{RolePlanner, 100, true, "main", "staging"},
		{RoleReviewer, 100, true, "main", "staging"},
		{RoleMerge, 100, true, "main", "staging"},
		{RoleDeployment, 100, true, "main", "staging"},
		{RoleGroomer, 100, true, "main", "staging"},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			cfg := DefaultConfig(tt.role)

			if cfg.Role != tt.role {
				t.Errorf("DefaultConfig(%q).Role = %q, want %q", tt.role, cfg.Role, tt.role)
			}
			if cfg.MaxIterations != tt.wantMaxIter {
				t.Errorf("DefaultConfig(%q).MaxIterations = %d, want %d", tt.role, cfg.MaxIterations, tt.wantMaxIter)
			}
			if cfg.SkipPerms != tt.wantSkipPerms {
				t.Errorf("DefaultConfig(%q).SkipPerms = %v, want %v", tt.role, cfg.SkipPerms, tt.wantSkipPerms)
			}
			if cfg.Branch != tt.wantBranch {
				t.Errorf("DefaultConfig(%q).Branch = %q, want %q", tt.role, cfg.Branch, tt.wantBranch)
			}
			if cfg.Environment != tt.wantEnvironment {
				t.Errorf("DefaultConfig(%q).Environment = %q, want %q", tt.role, cfg.Environment, tt.wantEnvironment)
			}
		})
	}
}

func TestMCPServers(t *testing.T) {
	tests := []struct {
		name  string
		role  Role
		extra []string
		want  []string
	}{
		{
			name:  "planner gets no base servers",
			role:  RolePlanner,
			extra: nil,
			want:  nil,
		},
		{
			name:  "reviewer gets no base servers",
			role:  RoleReviewer,
			extra: nil,
			want:  nil,
		},
		{
			name:  "groomer gets no base servers",
			role:  RoleGroomer,
			extra: nil,
			want:  nil,
		},
		{
			name:  "merge gets sequential-thinking",
			role:  RoleMerge,
			extra: nil,
			want:  []string{"sequential-thinking"},
		},
		{
			name:  "deployment gets sequential-thinking",
			role:  RoleDeployment,
			extra: nil,
			want:  []string{"sequential-thinking"},
		},
		{
			name:  "extra servers are appended",
			role:  RolePlanner,
			extra: []string{"github", "jira"},
			want:  []string{"github", "jira"},
		},
		{
			name:  "merge with extra servers",
			role:  RoleMerge,
			extra: []string{"github"},
			want:  []string{"sequential-thinking", "github"},
		},
		{
			name:  "empty extra servers",
			role:  RolePlanner,
			extra: []string{},
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MCPServers(tt.role, tt.extra)

			if len(got) != len(tt.want) {
				t.Errorf("MCPServers(%q, %v) returned %d servers, want %d\ngot:  %v\nwant: %v",
					tt.role, tt.extra, len(got), len(tt.want), got, tt.want)
				return
			}

			for i, server := range tt.want {
				if got[i] != server {
					t.Errorf("MCPServers(%q, %v)[%d] = %q, want %q",
						tt.role, tt.extra, i, got[i], server)
				}
			}
		})
	}
}

func TestConfig_Validate_EmptyWorkDir(t *testing.T) {
	cfg := Config{
		Role:    RolePlanner,
		WorkDir: "",
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should return error for empty WorkDir")
		return
	}

	if err.Error() != "working directory is required" {
		t.Errorf("Validate() error = %q, want %q", err.Error(), "working directory is required")
	}
}

func TestGetWorkDir(t *testing.T) {
	// GetWorkDir should return the current directory
	expected, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() failed: %v", err)
	}

	got, err := GetWorkDir()
	if err != nil {
		t.Errorf("GetWorkDir() error = %v", err)
		return
	}

	if got != expected {
		t.Errorf("GetWorkDir() = %q, want %q", got, expected)
	}
}

func TestPrintBanner(t *testing.T) {
	tests := []struct {
		role         Role
		wantContains []string
	}{
		{
			role: RolePlanner,
			wantContains: []string{
				"Starting Planner session...",
				"Issue tracker: beads",
			},
		},
		{
			role: RoleReviewer,
			wantContains: []string{
				"Starting Reviewer session...",
				"Issue tracker: beads",
			},
		},
		{
			role: RoleMerge,
			wantContains: []string{
				"Starting Merge Leader session...",
				"Issue tracker: beads",
			},
		},
		{
			role: RoleDeployment,
			wantContains: []string{
				"Starting Deployment Leader session...",
				"Issue tracker: beads",
			},
		},
		{
			role: RoleGroomer,
			wantContains: []string{
				"Starting Backlog Groomer session...",
				"Issue tracker: beads",
			},
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			printBanner(tt.role)

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := buf.String()

			for _, want := range tt.wantContains {
				if !bytes.Contains([]byte(output), []byte(want)) {
					t.Errorf("printBanner(%q) output missing %q\ngot: %q",
						tt.role, want, output)
				}
			}
		})
	}
}

func TestRoleConstants(t *testing.T) {
	// Verify role constants have expected string values
	tests := []struct {
		role Role
		want string
	}{
		{RolePlanner, "planner"},
		{RoleReviewer, "reviewer"},
		{RoleMerge, "merge"},
		{RoleDeployment, "deployment"},
		{RoleGroomer, "groomer"},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			if string(tt.role) != tt.want {
				t.Errorf("Role constant %q has value %q, want %q", tt.role, string(tt.role), tt.want)
			}
		})
	}
}

func TestConfigFields(t *testing.T) {
	// Test that Config fields are properly set and accessible
	cfg := Config{
		Role:          RoleMerge,
		WorkDir:       "/test/path",
		SessionID:     "session-123",
		SkipPerms:     true,
		MaxIterations: 50,
		ExtraServers:  []string{"github"},
		Branch:        "develop",
		Environment:   "production",
		DryRun:        true,
	}

	if cfg.Role != RoleMerge {
		t.Errorf("Config.Role = %q, want %q", cfg.Role, RoleMerge)
	}
	if cfg.WorkDir != "/test/path" {
		t.Errorf("Config.WorkDir = %q, want %q", cfg.WorkDir, "/test/path")
	}
	if cfg.SessionID != "session-123" {
		t.Errorf("Config.SessionID = %q, want %q", cfg.SessionID, "session-123")
	}
	if !cfg.SkipPerms {
		t.Error("Config.SkipPerms = false, want true")
	}
	if cfg.MaxIterations != 50 {
		t.Errorf("Config.MaxIterations = %d, want %d", cfg.MaxIterations, 50)
	}
	if len(cfg.ExtraServers) != 1 || cfg.ExtraServers[0] != "github" {
		t.Errorf("Config.ExtraServers = %v, want [github]", cfg.ExtraServers)
	}
	if cfg.Branch != "develop" {
		t.Errorf("Config.Branch = %q, want %q", cfg.Branch, "develop")
	}
	if cfg.Environment != "production" {
		t.Errorf("Config.Environment = %q, want %q", cfg.Environment, "production")
	}
	if !cfg.DryRun {
		t.Error("Config.DryRun = false, want true")
	}
}

func TestDefaultConfig_ZeroValues(t *testing.T) {
	// Test that DefaultConfig doesn't set fields that should be empty
	cfg := DefaultConfig(RolePlanner)

	if cfg.WorkDir != "" {
		t.Errorf("DefaultConfig().WorkDir = %q, want empty string", cfg.WorkDir)
	}
	if cfg.SessionID != "" {
		t.Errorf("DefaultConfig().SessionID = %q, want empty string", cfg.SessionID)
	}
	if len(cfg.ExtraServers) != 0 {
		t.Errorf("DefaultConfig().ExtraServers = %v, want empty slice", cfg.ExtraServers)
	}
	if cfg.DryRun {
		t.Error("DefaultConfig().DryRun = true, want false")
	}
}

func TestMCPServers_NoMutation(t *testing.T) {
	// Verify that MCPServers doesn't mutate the extra slice
	extra := []string{"github", "slack"}
	originalLen := len(extra)

	_ = MCPServers(RolePlanner, extra)

	if len(extra) != originalLen {
		t.Errorf("MCPServers mutated extra slice: len was %d, now %d", originalLen, len(extra))
	}
}

// ============================================================================
// Promise Constants Tests (promises.go)
// ============================================================================

func TestPromiseConstants(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"PromisePlanner", PromisePlanner, "PLANNER_COMPLETE"},
		{"PromiseReviewer", PromiseReviewer, "REVIEWER_COMPLETE"},
		{"PromiseMerge", PromiseMerge, "MERGE_COMPLETE"},
		{"PromiseDeploy", PromiseDeploy, "DEPLOY_COMPLETE"},
		{"PromiseSync", PromiseSync, "SYNC_COMPLETE"},
		{"PromiseGitHubSync", PromiseGitHubSync, "GITHUB_SYNCED"},
		{"PromiseNotionSync", PromiseNotionSync, "NOTION_SYNCED"},
		{"PromiseAnalyzer", PromiseAnalyzer, "ANALYZER_COMPLETE"},
		{"PromiseGroomer", PromiseGroomer, "GROOMER_COMPLETE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.value, tt.want)
			}
		})
	}
}

func TestWorkerPromiseFormat(t *testing.T) {
	if WorkerPromiseFormat != "WORKER_%s_COMPLETE" {
		t.Errorf("WorkerPromiseFormat = %q, want %q", WorkerPromiseFormat, "WORKER_%s_COMPLETE")
	}
}

func TestOutputFormatContents(t *testing.T) {
	// OutputFormat should contain key sections
	expectedContents := []string{
		"Output Format",
		"Status Reports",
		"Completion",
	}

	for _, expected := range expectedContents {
		if !bytes.Contains([]byte(OutputFormat), []byte(expected)) {
			t.Errorf("OutputFormat missing expected content: %q", expected)
		}
	}
}

func TestHandoffProtocolContents(t *testing.T) {
	// HandoffProtocol should contain key sections
	expectedContents := []string{
		"Handoff Protocol",
		"add_memory",
		"group_id",
		"forge-handoff",
	}

	for _, expected := range expectedContents {
		if !bytes.Contains([]byte(HandoffProtocol), []byte(expected)) {
			t.Errorf("HandoffProtocol missing expected content: %q", expected)
		}
	}
}

func TestErrorHandlingGuidanceContents(t *testing.T) {
	expectedContents := []string{
		"Error Handling",
		"API Rate Limits",
		"Partial Failures",
		"When Blocked",
	}

	for _, expected := range expectedContents {
		if !bytes.Contains([]byte(ErrorHandlingGuidance), []byte(expected)) {
			t.Errorf("ErrorHandlingGuidance missing expected content: %q", expected)
		}
	}
}

func TestSequentialThinkingTriggersContents(t *testing.T) {
	expectedContents := []string{
		"Sequential Thinking",
		"DO use sequential thinking",
		"Do NOT use sequential thinking",
	}

	for _, expected := range expectedContents {
		if !bytes.Contains([]byte(SequentialThinkingTriggers), []byte(expected)) {
			t.Errorf("SequentialThinkingTriggers missing expected content: %q", expected)
		}
	}
}

// ============================================================================
// TaskConfig Tests (taskconfig.go)
// ============================================================================

func TestTaskConfig_ToJSON(t *testing.T) {
	tc := &TaskConfig{
		Worker: WorkerIdentity{
			Name:   "alpha",
			ID:     "w-123",
			Role:   "worker",
			TaskID: "task-456",
		},
		Task: TaskDetails{
			Title:       "Test Task",
			Description: "Test description",
			Priority:    "high",
			Labels:      []string{"bug", "urgent"},
		},
		Workflow: WorkflowRules{
			Prohibited: []string{"don't do X"},
			Required:   []string{"must do Y"},
			Worktree:   "/path/to/worktree",
			Branch:     "feature/test",
		},
		Completion: CompletionConfig{
			Promise: "TEST_COMPLETE",
			Format:  "<promise>TEST_COMPLETE</promise>",
		},
	}

	json, err := tc.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	// Verify key fields are in the JSON
	expectedContents := []string{
		`"name": "alpha"`,
		`"id": "w-123"`,
		`"title": "Test Task"`,
		`"priority": "high"`,
		`"promise": "TEST_COMPLETE"`,
	}

	for _, expected := range expectedContents {
		if !bytes.Contains([]byte(json), []byte(expected)) {
			t.Errorf("ToJSON() missing expected content: %q\ngot: %s", expected, json)
		}
	}
}

func TestTaskConfig_ToPrompt(t *testing.T) {
	tc := &TaskConfig{
		Worker: WorkerIdentity{
			Name:   "bravo",
			ID:     "w-789",
			Role:   "developer",
			TaskID: "task-123",
		},
		Task: TaskDetails{
			Title:       "Implement Feature",
			Description: "Detailed feature description here",
			Priority:    "medium",
		},
		Workflow: WorkflowRules{
			Prohibited: []string{"merge to main", "skip tests"},
			Required:   []string{"write tests", "commit changes"},
			Worktree:   "/worktree/path",
			Branch:     "feature/new",
		},
		Completion: CompletionConfig{
			Promise: "FEATURE_DONE",
			Format:  "<promise>FEATURE_DONE</promise>",
		},
	}

	prompt := tc.ToPrompt()

	expectedContents := []string{
		"Task Configuration",
		"Instructions",
		"You are bravo (ID: w-789), a developer",
		"**Task**: Implement Feature",
		"Detailed feature description here",
		"**Do NOT**:",
		"merge to main",
		"skip tests",
		"**You MUST**:",
		"write tests",
		"commit changes",
		"**Completion**:",
		"FEATURE_DONE",
	}

	for _, expected := range expectedContents {
		if !bytes.Contains([]byte(prompt), []byte(expected)) {
			t.Errorf("ToPrompt() missing expected content: %q", expected)
		}
	}
}

func TestTaskConfig_ToPrompt_NoDescription(t *testing.T) {
	tc := &TaskConfig{
		Worker: WorkerIdentity{
			Name: "charlie",
			ID:   "w-abc",
			Role: "tester",
		},
		Task: TaskDetails{
			Title:       "Run Tests",
			Description: "", // Empty description
			Priority:    "low",
		},
		Completion: CompletionConfig{
			Promise: "TESTS_DONE",
			Format:  "<promise>TESTS_DONE</promise>",
		},
	}

	prompt := tc.ToPrompt()

	// Should still produce valid prompt without description section
	if !bytes.Contains([]byte(prompt), []byte("**Task**: Run Tests")) {
		t.Error("ToPrompt() should contain task title")
	}
}

func TestTaskConfig_ToPrompt_NoWorkflowRules(t *testing.T) {
	tc := &TaskConfig{
		Worker: WorkerIdentity{
			Name: "delta",
			ID:   "w-def",
			Role: "worker",
		},
		Task: TaskDetails{
			Title:    "Simple Task",
			Priority: "medium",
		},
		Workflow: WorkflowRules{
			Prohibited: []string{}, // Empty
			Required:   []string{}, // Empty
		},
		Completion: CompletionConfig{
			Promise: "DONE",
			Format:  "<promise>DONE</promise>",
		},
	}

	prompt := tc.ToPrompt()

	// Should not contain "Do NOT" or "You MUST" sections when empty
	if bytes.Contains([]byte(prompt), []byte("**Do NOT**:")) {
		t.Error("ToPrompt() should not contain 'Do NOT' section when prohibited is empty")
	}
	if bytes.Contains([]byte(prompt), []byte("**You MUST**:")) {
		t.Error("ToPrompt() should not contain 'You MUST' section when required is empty")
	}
}

func TestNewTaskConfig(t *testing.T) {
	tc := NewTaskConfig(
		"echo",
		"w-xyz",
		"worker",
		"task-999",
		"Build Feature",
		"Build the new feature",
		"high",
		"BUILD_COMPLETE",
		"/worktrees/feature",
		"feature/build",
	)

	// Verify worker identity
	if tc.Worker.Name != "echo" {
		t.Errorf("Worker.Name = %q, want %q", tc.Worker.Name, "echo")
	}
	if tc.Worker.ID != "w-xyz" {
		t.Errorf("Worker.ID = %q, want %q", tc.Worker.ID, "w-xyz")
	}
	if tc.Worker.Role != "worker" {
		t.Errorf("Worker.Role = %q, want %q", tc.Worker.Role, "worker")
	}
	if tc.Worker.TaskID != "task-999" {
		t.Errorf("Worker.TaskID = %q, want %q", tc.Worker.TaskID, "task-999")
	}

	// Verify task details
	if tc.Task.Title != "Build Feature" {
		t.Errorf("Task.Title = %q, want %q", tc.Task.Title, "Build Feature")
	}
	if tc.Task.Description != "Build the new feature" {
		t.Errorf("Task.Description = %q, want %q", tc.Task.Description, "Build the new feature")
	}
	if tc.Task.Priority != "high" {
		t.Errorf("Task.Priority = %q, want %q", tc.Task.Priority, "high")
	}

	// Verify workflow rules have defaults
	if len(tc.Workflow.Prohibited) != 3 {
		t.Errorf("Workflow.Prohibited length = %d, want 3", len(tc.Workflow.Prohibited))
	}
	if len(tc.Workflow.Required) != 3 {
		t.Errorf("Workflow.Required length = %d, want 3", len(tc.Workflow.Required))
	}
	if tc.Workflow.Worktree != "/worktrees/feature" {
		t.Errorf("Workflow.Worktree = %q, want %q", tc.Workflow.Worktree, "/worktrees/feature")
	}
	if tc.Workflow.Branch != "feature/build" {
		t.Errorf("Workflow.Branch = %q, want %q", tc.Workflow.Branch, "feature/build")
	}

	// Verify completion
	if tc.Completion.Promise != "BUILD_COMPLETE" {
		t.Errorf("Completion.Promise = %q, want %q", tc.Completion.Promise, "BUILD_COMPLETE")
	}
	if tc.Completion.Format != "<promise>BUILD_COMPLETE</promise>" {
		t.Errorf("Completion.Format = %q, want %q", tc.Completion.Format, "<promise>BUILD_COMPLETE</promise>")
	}
}

func TestSupervisorMessage_ToJSON(t *testing.T) {
	msg := SupervisorMessage{
		Type:    "poke",
		Count:   3,
		Message: "Please wrap up soon",
		Action:  "finalize",
	}

	json := msg.ToJSON()

	expectedContents := []string{
		`"type":"poke"`,
		`"count":3`,
		`"message":"Please wrap up soon"`,
		`"action":"finalize"`,
	}

	for _, expected := range expectedContents {
		if !bytes.Contains([]byte(json), []byte(expected)) {
			t.Errorf("ToJSON() missing expected content: %q\ngot: %s", expected, json)
		}
	}
}

func TestBuildPokeMessage(t *testing.T) {
	tests := []struct {
		pokeCount   int
		wantMessage string
		wantAction  string
	}{
		{1, "Status check", "continue"},
		{2, "Status check", "continue"},
		{3, "Please wrap up soon", "finalize"},
		{4, "Please wrap up soon", "finalize"},
		{5, "Time to finish", "wrap_up"},
		{6, "Time to finish", "wrap_up"},
		{7, "Time to finish", "wrap_up"},
		{8, "Urgent: complete immediately", "output_promise"},
		{10, "Urgent: complete immediately", "output_promise"},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.pokeCount)), func(t *testing.T) {
			result := BuildPokeMessage(tt.pokeCount)

			// Result should be an echo command with JSON
			if !bytes.HasPrefix([]byte(result), []byte("echo '")) {
				t.Errorf("BuildPokeMessage(%d) should start with echo command, got: %s", tt.pokeCount, result)
			}

			if !bytes.Contains([]byte(result), []byte(tt.wantMessage)) {
				t.Errorf("BuildPokeMessage(%d) missing message %q, got: %s", tt.pokeCount, tt.wantMessage, result)
			}

			if !bytes.Contains([]byte(result), []byte(tt.wantAction)) {
				t.Errorf("BuildPokeMessage(%d) missing action %q, got: %s", tt.pokeCount, tt.wantAction, result)
			}
		})
	}
}

// ============================================================================
// Leader Prompt Tests (planner.go, reviewer.go, merge.go, deploy.go)
// ============================================================================

func TestPlannerPrompt(t *testing.T) {
	prompt := PlannerPrompt("db-123", "/work/dir")

	expectedContents := []string{
		"Forge Planner",
		"db-123",
		"/work/dir",
		"Planning Workflow",
		"Phase 1: Context Gathering",
		"Phase 2: Planning",
		"Phase 3: Prioritization",
		OutputFormat[:50],    // Check first 50 chars of OutputFormat
		HandoffProtocol[:50], // Check first 50 chars of HandoffProtocol
	}

	for _, expected := range expectedContents {
		if !bytes.Contains([]byte(prompt), []byte(expected)) {
			t.Errorf("PlannerPrompt() missing expected content: %q", expected)
		}
	}
}

func TestPlannerPromise(t *testing.T) {
	if got := PlannerPromise(); got != PromisePlanner {
		t.Errorf("PlannerPromise() = %q, want %q", got, PromisePlanner)
	}
}

func TestReviewerPrompt(t *testing.T) {
	prompt := ReviewerPrompt("db-456", "/review/dir", "main")

	expectedContents := []string{
		"Forge Reviewer",
		"db-456",
		"/review/dir",
		"Review Branch: main",
		"Review Workflow",
		"Phase 1: Understand the Changes",
		"Phase 2: Review Checklist",
		"Phase 3: Run Automated Checks",
		"Phase 4: Create Issues",
	}

	for _, expected := range expectedContents {
		if !bytes.Contains([]byte(prompt), []byte(expected)) {
			t.Errorf("ReviewerPrompt() missing expected content: %q", expected)
		}
	}
}

func TestReviewerPromise(t *testing.T) {
	if got := ReviewerPromise(); got != PromiseReviewer {
		t.Errorf("ReviewerPromise() = %q, want %q", got, PromiseReviewer)
	}
}

func TestMergePrompt(t *testing.T) {
	prompt := MergePrompt("db-789", "/merge/dir", "develop", false)

	expectedContents := []string{
		"Forge Merge Leader",
		"db-789",
		"/merge/dir",
		"Target Branch: develop",
		"Merge Workflow",
		"Phase 1: Assess Merge Queue",
		"Phase 2: Pre-Merge Checks",
		"Phase 3: Execute Merge",
		"Phase 4: Post-Merge Updates",
	}

	for _, expected := range expectedContents {
		if !bytes.Contains([]byte(prompt), []byte(expected)) {
			t.Errorf("MergePrompt() missing expected content: %q", expected)
		}
	}

	// Should NOT contain dry run note
	if bytes.Contains([]byte(prompt), []byte("DRY RUN MODE")) {
		t.Error("MergePrompt(dryRun=false) should not contain DRY RUN MODE")
	}
}

func TestMergePrompt_DryRun(t *testing.T) {
	prompt := MergePrompt("db-789", "/merge/dir", "main", true)

	// Should contain dry run note
	if !bytes.Contains([]byte(prompt), []byte("DRY RUN MODE")) {
		t.Error("MergePrompt(dryRun=true) should contain DRY RUN MODE")
	}

	if !bytes.Contains([]byte(prompt), []byte("Do NOT actually execute merges")) {
		t.Error("MergePrompt(dryRun=true) should contain dry run instructions")
	}
}

func TestMergePromise(t *testing.T) {
	if got := MergePromise(); got != PromiseMerge {
		t.Errorf("MergePromise() = %q, want %q", got, PromiseMerge)
	}
}

func TestDeployPrompt(t *testing.T) {
	prompt := DeployPrompt("db-012", "/deploy/dir", "staging", false)

	expectedContents := []string{
		"Forge Deployment Leader",
		"db-012",
		"/deploy/dir",
		"Target Environment: staging",
		"Deployment Workflow",
		"Phase 1: Pre-Deployment Assessment",
		"Phase 2: Build and Test",
		"Phase 3: Deploy",
		"Phase 4: Smoke Tests",
		"Phase 5: Post-Deployment",
	}

	for _, expected := range expectedContents {
		if !bytes.Contains([]byte(prompt), []byte(expected)) {
			t.Errorf("DeployPrompt() missing expected content: %q", expected)
		}
	}

	// Should NOT contain dry run note
	if bytes.Contains([]byte(prompt), []byte("DRY RUN MODE")) {
		t.Error("DeployPrompt(dryRun=false) should not contain DRY RUN MODE")
	}
}

func TestDeployPrompt_DryRun(t *testing.T) {
	prompt := DeployPrompt("db-012", "/deploy/dir", "production", true)

	// Should contain dry run note
	if !bytes.Contains([]byte(prompt), []byte("DRY RUN MODE")) {
		t.Error("DeployPrompt(dryRun=true) should contain DRY RUN MODE")
	}

	if !bytes.Contains([]byte(prompt), []byte("Do NOT actually deploy")) {
		t.Error("DeployPrompt(dryRun=true) should contain dry run instructions")
	}
}

func TestDeployPromise(t *testing.T) {
	if got := DeployPromise(); got != PromiseDeploy {
		t.Errorf("DeployPromise() = %q, want %q", got, PromiseDeploy)
	}
}
