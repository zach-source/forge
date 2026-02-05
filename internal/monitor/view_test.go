package monitor

import (
	"strings"
	"testing"
	"time"

	"github.com/zach-source/forge/internal/kanban"
	"github.com/zach-source/forge/internal/logs"
	"github.com/zach-source/forge/internal/ralph"
	"github.com/zach-source/forge/internal/session"
	"github.com/zach-source/forge/internal/worker"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		want     string
	}{
		{30 * time.Second, "30s"},
		{59 * time.Second, "59s"},
		{1 * time.Minute, "1m"},
		{5 * time.Minute, "5m"},
		{59 * time.Minute, "59m"},
		{1 * time.Hour, "1h"},
		{5 * time.Hour, "5h"},
		{23 * time.Hour, "23h"},
		{24 * time.Hour, "1d"},
		{48 * time.Hour, "2d"},
		{72 * time.Hour, "3d"},
	}

	for _, tt := range tests {
		got := formatDuration(tt.duration)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, got, tt.want)
		}
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0B"},
		{100, "100B"},
		{1023, "1023B"},
		{1024, "1.0KB"},
		{1536, "1.5KB"},
		{1024 * 1024, "1.0MB"},
		{1024 * 1024 * 1024, "1.0GB"},
		{1536 * 1024, "1.5MB"},
	}

	for _, tt := range tests {
		got := formatSize(tt.bytes)
		if got != tt.want {
			t.Errorf("formatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestStatusIcon(t *testing.T) {
	tests := []struct {
		status kanban.Status
		want   string
	}{
		{kanban.StatusBacklog, "📥"},
		{kanban.StatusTodo, "📝"},
		{kanban.StatusInProgress, "🔄"},
		{kanban.StatusReview, "🔍"},
		{kanban.StatusDone, "✅"},
		{kanban.Status("unknown"), "❓"},
	}

	for _, tt := range tests {
		got := statusIcon(tt.status)
		if got != tt.want {
			t.Errorf("statusIcon(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestWorkerStatusIcon(t *testing.T) {
	tests := []struct {
		status worker.Status
		want   string
	}{
		{worker.StatusIdle, "💤"},
		{worker.StatusActive, "🔄"},
		{worker.StatusPaused, "⏸️"},
		{worker.StatusStopped, "🛑"},
		{worker.Status("unknown"), "❓"},
	}

	for _, tt := range tests {
		got := workerStatusIcon(tt.status)
		if got != tt.want {
			t.Errorf("workerStatusIcon(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestWorkerRoleIcon(t *testing.T) {
	tests := []struct {
		role worker.Role
		want string
	}{
		{worker.RoleWorker, "👷"},
		{worker.RolePlanner, "📋"},
		{worker.RoleReviewer, "🔍"},
		{worker.RoleMerge, "🔀"},
		{worker.RoleDeploy, "🚀"},
		{worker.RoleGroomer, "🧹"},
		{worker.Role("unknown"), "❓"},
	}

	for _, tt := range tests {
		got := workerRoleIcon(tt.role)
		if got != tt.want {
			t.Errorf("workerRoleIcon(%q) = %q, want %q", tt.role, got, tt.want)
		}
	}
}

func TestLogTypeIcon(t *testing.T) {
	tests := []struct {
		logType logs.LogType
		want    string
	}{
		{logs.LogTypeSession, "📄"},
		{logs.LogTypeWorker, "👷"},
		{logs.LogTypeLeader, "👑"},
		{logs.LogType("unknown"), "📄"},
	}

	for _, tt := range tests {
		got := logTypeIcon(tt.logType)
		if got != tt.want {
			t.Errorf("logTypeIcon(%q) = %q, want %q", tt.logType, got, tt.want)
		}
	}
}

func TestRenderHeader(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80

	header := m.renderHeader()

	if !strings.Contains(header, "foundry monitor") {
		t.Error("Header should contain 'foundry monitor'")
	}
	if !strings.Contains(header, "workers") && !strings.Contains(header, "sessions") {
		t.Error("Header should contain summary counts")
	}
}

func TestRenderHeader_WithActiveSessions(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80
	m.sessions.Add(&session.Session{
		ID:     "test-1",
		Status: session.StatusActive,
	})
	m.workers = []*worker.Worker{
		{ID: "w-1", Name: "alpha", Status: worker.StatusActive},
	}

	header := m.renderHeader()

	if !strings.Contains(header, "active") {
		t.Error("Header should indicate active sessions/workers")
	}
}

func TestRenderTabs(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80
	m.activeTab = TabWorkers

	tabs := m.renderTabs()

	// Check all tab names are present
	for _, name := range tabNames() {
		if !strings.Contains(tabs, name) {
			t.Errorf("Tabs should contain %q", name)
		}
	}
}

func TestRenderOverviewTab(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80

	view := m.renderOverviewTab()

	if !strings.Contains(view, "Kanban") {
		t.Error("Overview tab should contain Kanban section")
	}
	if !strings.Contains(view, "Workers") {
		t.Error("Overview tab should contain Workers section")
	}
}

func TestRenderKanbanSummary_NoData(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80
	m.kanban = nil

	summary := m.renderKanbanSummary()

	if !strings.Contains(summary, "No kanban data") {
		t.Error("Should show no kanban data message")
	}
}

func TestRenderKanbanSummary_WithData(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80
	m.kanban = &kanban.Board{
		Columns: []kanban.Column{
			{Status: kanban.StatusTodo, Issues: []*kanban.Issue{{ID: "1"}, {ID: "2"}}},
			{Status: kanban.StatusInProgress, Issues: []*kanban.Issue{{ID: "3"}}},
		},
	}

	summary := m.renderKanbanSummary()

	if !strings.Contains(summary, "Todo") {
		t.Error("Should show Todo column")
	}
	if !strings.Contains(summary, "In Progress") {
		t.Error("Should show In Progress column")
	}
}

func TestRenderWorkerSummary_NoWorkers(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80
	m.workers = nil

	summary := m.renderWorkerSummary()

	if !strings.Contains(summary, "No workers") {
		t.Error("Should show no workers message")
	}
}

func TestRenderWorkerSummary_WithWorkers(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80
	m.workers = []*worker.Worker{
		{ID: "w-1", Name: "alpha", Status: worker.StatusActive, Role: worker.RoleWorker},
		{ID: "w-2", Name: "bravo", Status: worker.StatusIdle, Role: worker.RoleReviewer},
	}

	summary := m.renderWorkerSummary()

	if !strings.Contains(summary, "Status") {
		t.Error("Should show status section")
	}
	if !strings.Contains(summary, "Roles") {
		t.Error("Should show roles section")
	}
}

func TestRenderWorkersTab_NoWorkers(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80
	m.workers = nil

	view := m.renderWorkersTab()

	if !strings.Contains(view, "No workers") {
		t.Error("Should show no workers message")
	}
	if !strings.Contains(view, "foundry worker create") {
		t.Error("Should show create command hint")
	}
}

func TestRenderWorkersTab_WithWorkers(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80
	m.activeTab = TabWorkers
	m.workers = []*worker.Worker{
		{ID: "w-a1b2c3d4", Name: "alpha", Status: worker.StatusActive, Role: worker.RoleWorker, LastActive: time.Now()},
		{ID: "w-b2c3d4e5", Name: "bravo", Status: worker.StatusIdle, Role: worker.RoleReviewer, LastActive: time.Now()},
	}
	m.selected = 0

	view := m.renderWorkersTab()

	if !strings.Contains(view, "Workers") {
		t.Error("Should show Workers label")
	}
	if !strings.Contains(view, "alpha") {
		t.Error("Should show worker alpha")
	}
	if !strings.Contains(view, "bravo") {
		t.Error("Should show worker bravo")
	}
}

func TestRenderWorkerRow_Selected(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80
	m.activeTab = TabWorkers
	m.selected = 0

	w := &worker.Worker{
		ID:          "w-a1b2c3d4",
		Name:        "alpha",
		Status:      worker.StatusActive,
		Role:        worker.RoleWorker,
		CurrentTask: "task-123",
		LastActive:  time.Now(),
	}

	row := m.renderWorkerRow(0, w)

	if !strings.Contains(row, "▸") {
		t.Error("Selected row should have selection indicator")
	}
	if !strings.Contains(row, "alpha") {
		t.Error("Row should contain worker name")
	}
}

func TestRenderWorkerRow_NotSelected(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80
	m.activeTab = TabWorkers
	m.selected = 1 // Different selection

	w := &worker.Worker{
		ID:         "w-a1b2c3d4",
		Name:       "alpha",
		Status:     worker.StatusIdle,
		Role:       worker.RoleWorker,
		LastActive: time.Now(),
	}

	row := m.renderWorkerRow(0, w)

	if strings.Contains(row, "▸") {
		t.Error("Non-selected row should not have selection indicator")
	}
}

func TestRenderSessionsTab_NoSessions(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80

	view := m.renderSessionsTab()

	if !strings.Contains(view, "No forge sessions") {
		t.Error("Should show no sessions message")
	}
}

func TestRenderSessionsTab_WithSessions(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80
	m.activeTab = TabSessions
	m.sessions.Add(&session.Session{
		ID:        "test-session",
		Status:    session.StatusActive,
		StartedAt: time.Now(),
		State: &ralph.State{
			Iteration:         5,
			MaxIterations:     50,
			CompletionPromise: "DONE",
		},
	})

	view := m.renderSessionsTab()

	if !strings.Contains(view, "Sessions") {
		t.Error("Should show Sessions label")
	}
	if !strings.Contains(view, "test-session") {
		t.Error("Should show session ID")
	}
}

func TestRenderSessionRow_Selected(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80
	m.activeTab = TabSessions
	m.selected = 0

	s := &session.Session{
		ID:        "test-session",
		Status:    session.StatusActive,
		StartedAt: time.Now().Add(-5 * time.Minute),
		State: &ralph.State{
			Iteration:         5,
			MaxIterations:     50,
			CompletionPromise: "API_DONE",
		},
	}

	row := m.renderSessionRow(0, s)

	if !strings.Contains(row, "▸") {
		t.Error("Selected row should have selection indicator")
	}
	if !strings.Contains(row, "test-session") {
		t.Error("Row should contain session ID")
	}
}

func TestRenderOutput_NoSessions(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80

	output := m.renderOutput()

	if output != "" {
		t.Errorf("Expected empty output when no sessions, got %q", output)
	}
}

func TestRenderOutput_WithOutput(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80
	m.height = 40
	m.activeTab = TabSessions
	m.sessions.Add(&session.Session{
		ID:     "test-session",
		Status: session.StatusActive,
	})
	m.output = []string{
		"Line 1",
		"Line 2",
		"Line 3",
	}
	m.selected = 0

	output := m.renderOutput()

	if !strings.Contains(output, "Output") {
		t.Error("Should show Output label")
	}
	if !strings.Contains(output, "test-session") {
		t.Error("Should show session name in output header")
	}
}

func TestRenderOutput_NoOutputAvailable(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80
	m.height = 40
	m.activeTab = TabSessions
	m.sessions.Add(&session.Session{
		ID:     "test-session",
		Status: session.StatusActive,
	})
	m.output = nil
	m.selected = 0

	output := m.renderOutput()

	if !strings.Contains(output, "No output available") {
		t.Error("Should show no output message")
	}
}

func TestRenderLogsTab_NoLogs(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80
	m.logEntries = nil

	view := m.renderLogsTab()

	if !strings.Contains(view, "No log files") {
		t.Error("Should show no logs message")
	}
}

func TestRenderLogsTab_WithLogs(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80
	m.activeTab = TabLogs
	m.logEntries = []logs.LogEntry{
		{Name: "session-1", Type: logs.LogTypeSession, Size: 1024, ModTime: time.Now()},
		{Name: "worker-alpha", Type: logs.LogTypeWorker, Size: 2048, ModTime: time.Now()},
	}

	view := m.renderLogsTab()

	if !strings.Contains(view, "Log Files") {
		t.Error("Should show Log Files label")
	}
	if !strings.Contains(view, "session-1") {
		t.Error("Should show log entry name")
	}
}

func TestRenderLogRow_Selected(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80
	m.activeTab = TabLogs
	m.selected = 0

	entry := logs.LogEntry{
		Name:    "test-log",
		Type:    logs.LogTypeSession,
		Size:    1024,
		ModTime: time.Now(),
	}

	row := m.renderLogRow(0, entry)

	if !strings.Contains(row, "▸") {
		t.Error("Selected row should have selection indicator")
	}
	if !strings.Contains(row, "test-log") {
		t.Error("Row should contain log name")
	}
}

func TestRenderHelp(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80

	// Test help for different tabs
	tabs := []Tab{TabOverview, TabWorkers, TabSessions, TabLogs}

	for _, tab := range tabs {
		m.activeTab = tab
		help := m.renderHelp()

		if !strings.Contains(help, "tab") {
			t.Errorf("Help for tab %d should contain 'tab'", tab)
		}
		if !strings.Contains(help, "[q]uit") {
			t.Errorf("Help for tab %d should contain '[q]uit'", tab)
		}
	}
}

func TestRenderHelp_SessionsTab(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80
	m.activeTab = TabSessions

	help := m.renderHelp()

	if !strings.Contains(help, "[a]ttach") {
		t.Error("Sessions tab help should contain '[a]ttach'")
	}
	if !strings.Contains(help, "[c]ancel") {
		t.Error("Sessions tab help should contain '[c]ancel'")
	}
}

func TestRenderHelp_WorkersTab(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80
	m.activeTab = TabWorkers

	help := m.renderHelp()

	if !strings.Contains(help, "[a]ttach") {
		t.Error("Workers tab help should contain '[a]ttach'")
	}
}

func TestRenderView_AllTabs(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80
	m.height = 40

	tabs := []Tab{TabOverview, TabWorkers, TabSessions, TabLogs}

	for _, tab := range tabs {
		m.activeTab = tab
		view := m.renderView()

		if view == "" {
			t.Errorf("View for tab %d should not be empty", tab)
		}
		// Should always have header and help
		if !strings.Contains(view, "foundry monitor") {
			t.Errorf("View for tab %d should contain header", tab)
		}
	}
}

func TestTruncate_EdgeCases(t *testing.T) {
	// Test exact boundary
	got := truncate("abc", 3)
	if got != "abc" {
		t.Errorf("truncate('abc', 3) = %q, want 'abc'", got)
	}

	// Test one over boundary
	got = truncate("abcd", 3)
	if got != "..." {
		t.Errorf("truncate('abcd', 3) = %q, want '...'", got)
	}

	// Test empty string
	got = truncate("", 10)
	if got != "" {
		t.Errorf("truncate('', 10) = %q, want ''", got)
	}
}

func TestRenderSessionRow_LongPromise(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80
	m.activeTab = TabSessions
	m.selected = 0

	s := &session.Session{
		ID:        "test",
		Status:    session.StatusActive,
		StartedAt: time.Now(),
		State: &ralph.State{
			Iteration:         1,
			MaxIterations:     10,
			CompletionPromise: "THIS_IS_A_VERY_LONG_COMPLETION_PROMISE_THAT_SHOULD_BE_TRUNCATED",
		},
	}

	row := m.renderSessionRow(0, s)

	// Promise should be truncated
	if strings.Contains(row, "TRUNCATED") {
		t.Error("Long promise should be truncated")
	}
}

func TestRenderWorkerRow_WithTask(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80
	m.activeTab = TabWorkers
	m.selected = 0

	w := &worker.Worker{
		ID:          "w-a1b2c3d4",
		Name:        "alpha",
		Status:      worker.StatusActive,
		Role:        worker.RoleWorker,
		CurrentTask: "task-very-long-id-that-should-be-truncated",
		LastActive:  time.Now(),
	}

	row := m.renderWorkerRow(0, w)

	// Should contain truncated task (12 chars max: 9 + "...")
	if !strings.Contains(row, "task-very...") {
		t.Error("Row should contain truncated task")
	}
}

func TestRenderWorkerRow_NoTask(t *testing.T) {
	m := NewModel("/tmp")
	m.width = 80
	m.activeTab = TabWorkers
	m.selected = 0

	w := &worker.Worker{
		ID:          "w-a1b2c3d4",
		Name:        "alpha",
		Status:      worker.StatusIdle,
		Role:        worker.RoleWorker,
		CurrentTask: "",
		LastActive:  time.Now(),
	}

	row := m.renderWorkerRow(0, w)

	// Should show dash for no task
	if !strings.Contains(row, "-") {
		t.Error("Row should contain dash for no task")
	}
}
