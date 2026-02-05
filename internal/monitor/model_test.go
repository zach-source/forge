package monitor

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/zach-source/forge/internal/kanban"
	"github.com/zach-source/forge/internal/logs"
	"github.com/zach-source/forge/internal/ralph"
	"github.com/zach-source/forge/internal/session"
	"github.com/zach-source/forge/internal/worker"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"this is a long string", 10, "this is..."},
		{"exactly10!", 10, "exactly10!"},
		{"", 5, ""},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

func TestNewModel(t *testing.T) {
	m := NewModel("/tmp")

	if m.sessions == nil {
		t.Error("Expected sessions manager to be initialized")
	}
	if m.selected != 0 {
		t.Errorf("Expected selected to be 0, got %d", m.selected)
	}
	if m.workDir != "/tmp" {
		t.Errorf("Expected workDir to be /tmp, got %s", m.workDir)
	}
	if m.activeTab != TabOverview {
		t.Errorf("Expected activeTab to be TabOverview, got %d", m.activeTab)
	}
	if m.width != 80 {
		t.Errorf("Expected default width to be 80, got %d", m.width)
	}
	if m.height != 24 {
		t.Errorf("Expected default height to be 24, got %d", m.height)
	}
	if m.quitting {
		t.Error("Expected quitting to be false")
	}
}

func TestModel_Update_WindowSize(t *testing.T) {
	m := NewModel("/tmp")
	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	newModel, cmd := m.Update(msg)
	model := newModel.(Model)

	if model.width != 100 {
		t.Errorf("Expected width 100, got %d", model.width)
	}
	if model.height != 50 {
		t.Errorf("Expected height 50, got %d", model.height)
	}
	if cmd != nil {
		t.Error("Expected no command for WindowSizeMsg")
	}
}

func TestModel_Update_TabNavigation(t *testing.T) {
	m := NewModel("/tmp")

	// Test TabNext
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}
	newModel, _ := m.Update(keyMsg)
	model := newModel.(Model)
	if model.activeTab != TabWorkers {
		t.Errorf("Expected TabWorkers after TabNext, got %d", model.activeTab)
	}

	// Test TabNext wrapping
	model.activeTab = TabLogs
	newModel, _ = model.Update(keyMsg)
	model = newModel.(Model)
	if model.activeTab != TabOverview {
		t.Errorf("Expected TabOverview after wrapping, got %d", model.activeTab)
	}

	// Test TabPrev
	model.activeTab = TabWorkers
	keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}
	newModel, _ = model.Update(keyMsg)
	model = newModel.(Model)
	if model.activeTab != TabOverview {
		t.Errorf("Expected TabOverview after TabPrev, got %d", model.activeTab)
	}

	// Test TabPrev wrapping
	model.activeTab = TabOverview
	newModel, _ = model.Update(keyMsg)
	model = newModel.(Model)
	if model.activeTab != TabLogs {
		t.Errorf("Expected TabLogs after wrapping, got %d", model.activeTab)
	}
}

func TestModel_Update_TabNumbers(t *testing.T) {
	tests := []struct {
		key     rune
		wantTab Tab
	}{
		{'1', TabOverview},
		{'2', TabWorkers},
		{'3', TabSessions},
		{'4', TabLogs},
	}

	for _, tt := range tests {
		m := NewModel("/tmp")
		m.activeTab = TabLogs // Start at a different tab
		m.selected = 5        // Set non-zero selection

		keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tt.key}}
		newModel, _ := m.Update(keyMsg)
		model := newModel.(Model)

		if model.activeTab != tt.wantTab {
			t.Errorf("Key %c: expected tab %d, got %d", tt.key, tt.wantTab, model.activeTab)
		}
		if model.selected != 0 {
			t.Errorf("Key %c: expected selected to reset to 0, got %d", tt.key, model.selected)
		}
	}
}

func TestModel_Update_UpDown(t *testing.T) {
	m := NewModel("/tmp")
	m.activeTab = TabWorkers
	m.workers = []*worker.Worker{
		{ID: "w-1", Name: "alpha"},
		{ID: "w-2", Name: "bravo"},
		{ID: "w-3", Name: "charlie"},
	}
	m.selected = 0

	// Test Down
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	newModel, _ := m.Update(keyMsg)
	model := newModel.(Model)
	if model.selected != 1 {
		t.Errorf("Expected selected 1 after Down, got %d", model.selected)
	}

	// Test Down again
	newModel, _ = model.Update(keyMsg)
	model = newModel.(Model)
	if model.selected != 2 {
		t.Errorf("Expected selected 2 after second Down, got %d", model.selected)
	}

	// Test Down at end (should not go past last item)
	newModel, _ = model.Update(keyMsg)
	model = newModel.(Model)
	if model.selected != 2 {
		t.Errorf("Expected selected to stay at 2, got %d", model.selected)
	}

	// Test Up
	keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	newModel, _ = model.Update(keyMsg)
	model = newModel.(Model)
	if model.selected != 1 {
		t.Errorf("Expected selected 1 after Up, got %d", model.selected)
	}

	// Test Up at beginning
	model.selected = 0
	newModel, _ = model.Update(keyMsg)
	model = newModel.(Model)
	if model.selected != 0 {
		t.Errorf("Expected selected to stay at 0, got %d", model.selected)
	}
}

func TestModel_Update_Quit(t *testing.T) {
	m := NewModel("/tmp")

	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	newModel, cmd := m.Update(keyMsg)
	model := newModel.(Model)

	if !model.quitting {
		t.Error("Expected quitting to be true")
	}
	if cmd == nil {
		t.Error("Expected tea.Quit command")
	}
}

func TestModel_Update_DataRefreshed(t *testing.T) {
	m := NewModel("/tmp")

	workers := []*worker.Worker{
		{ID: "w-1", Name: "alpha", Status: worker.StatusActive},
	}
	board := &kanban.Board{
		Columns: []kanban.Column{
			{Status: kanban.StatusTodo, Issues: []*kanban.Issue{{ID: "1"}}},
		},
	}
	logEntries := []logs.LogEntry{
		{Name: "test.log", Type: logs.LogTypeSession},
	}

	msg := dataRefreshedMsg{
		sessions:   nil,
		workers:    workers,
		kanban:     board,
		logEntries: logEntries,
	}

	newModel, _ := m.Update(msg)
	model := newModel.(Model)

	if len(model.workers) != 1 {
		t.Errorf("Expected 1 worker, got %d", len(model.workers))
	}
	if model.kanban == nil {
		t.Error("Expected kanban to be set")
	}
	if len(model.logEntries) != 1 {
		t.Errorf("Expected 1 log entry, got %d", len(model.logEntries))
	}
}

func TestModel_Update_ClearErrorOnKeypress(t *testing.T) {
	m := NewModel("/tmp")
	m.error = "some error"

	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	newModel, _ := m.Update(keyMsg)
	model := newModel.(Model)

	if model.error != "" {
		t.Errorf("Expected error to be cleared, got %s", model.error)
	}
}

func TestModel_MaxItemsForTab(t *testing.T) {
	m := NewModel("/tmp")
	m.workers = []*worker.Worker{
		{ID: "w-1"},
		{ID: "w-2"},
	}
	m.logEntries = []logs.LogEntry{
		{Name: "log1"},
		{Name: "log2"},
		{Name: "log3"},
	}
	// Add sessions to manager
	m.sessions.Add(&session.Session{ID: "s-1"})
	m.sessions.Add(&session.Session{ID: "s-2"})
	m.sessions.Add(&session.Session{ID: "s-3"})
	m.sessions.Add(&session.Session{ID: "s-4"})

	tests := []struct {
		tab  Tab
		want int
	}{
		{TabOverview, 0},
		{TabWorkers, 2},
		{TabSessions, 4},
		{TabLogs, 3},
	}

	for _, tt := range tests {
		m.activeTab = tt.tab
		got := m.maxItemsForTab()
		if got != tt.want {
			t.Errorf("maxItemsForTab() for tab %d: got %d, want %d", tt.tab, got, tt.want)
		}
	}
}

func TestModel_View_Quitting(t *testing.T) {
	m := NewModel("/tmp")
	m.quitting = true

	view := m.View()
	if view != "" {
		t.Errorf("Expected empty view when quitting, got %q", view)
	}
}

func TestModel_View_Attaching(t *testing.T) {
	m := NewModel("/tmp")
	m.attaching = "forge-test"

	view := m.View()
	if view != "Attaching to forge-test...\n" {
		t.Errorf("Expected attaching message, got %q", view)
	}
}

func TestTabNames(t *testing.T) {
	names := tabNames()
	expected := []string{"Overview", "Workers", "Sessions", "Logs"}

	if len(names) != len(expected) {
		t.Fatalf("Expected %d tab names, got %d", len(expected), len(names))
	}

	for i, name := range names {
		if name != expected[i] {
			t.Errorf("Tab name %d: got %q, want %q", i, name, expected[i])
		}
	}
}

func TestModel_HandleAttach_Sessions(t *testing.T) {
	m := NewModel("/tmp")
	m.activeTab = TabSessions
	m.sessions.Add(&session.Session{
		ID:   "test-session",
		Tmux: "forge-test",
	})
	m.selected = 0

	cmd := m.handleAttach()
	if cmd == nil {
		t.Error("Expected command for attach")
	}

	// Execute the command and check the message
	msg := cmd()
	attachMsg, ok := msg.(attachMsg)
	if !ok {
		t.Fatal("Expected attachMsg")
	}
	if attachMsg.sessionName != "forge-test" {
		t.Errorf("Expected session name forge-test, got %s", attachMsg.sessionName)
	}
}

func TestModel_HandleAttach_Workers(t *testing.T) {
	m := NewModel("/tmp")
	m.activeTab = TabWorkers
	m.workers = []*worker.Worker{
		{ID: "w-a1b2c3d4", Name: "alpha", SessionID: "forge-alpha"},
	}
	m.selected = 0

	cmd := m.handleAttach()
	if cmd == nil {
		t.Error("Expected command for attach")
	}

	msg := cmd()
	attachMsg, ok := msg.(attachMsg)
	if !ok {
		t.Fatal("Expected attachMsg")
	}
	// TmuxSessionName format: forge-{name}-{shortid}
	expectedName := "forge-alpha-a1b2c3d4"
	if attachMsg.sessionName != expectedName {
		t.Errorf("Expected session name %s, got %s", expectedName, attachMsg.sessionName)
	}
}

func TestModel_HandleAttach_NoSession(t *testing.T) {
	m := NewModel("/tmp")
	m.activeTab = TabSessions
	// No sessions added

	cmd := m.handleAttach()
	if cmd != nil {
		t.Error("Expected nil command when no sessions")
	}
}

func TestModel_HandleAttach_NoTmux(t *testing.T) {
	m := NewModel("/tmp")
	m.activeTab = TabSessions
	m.sessions.Add(&session.Session{
		ID:   "test-session",
		Tmux: "", // No tmux session
	})
	m.selected = 0

	cmd := m.handleAttach()
	if cmd != nil {
		t.Error("Expected nil command when no tmux session")
	}
}

func TestModel_HandleAttach_WorkerNoSession(t *testing.T) {
	m := NewModel("/tmp")
	m.activeTab = TabWorkers
	m.workers = []*worker.Worker{
		{ID: "w-1", Name: "alpha", SessionID: ""}, // No session
	}
	m.selected = 0

	cmd := m.handleAttach()
	if cmd != nil {
		t.Error("Expected nil command when worker has no session")
	}
}

func TestModel_HandleCancel_Sessions(t *testing.T) {
	m := NewModel("/tmp")
	m.activeTab = TabSessions
	m.sessions.Add(&session.Session{
		ID:   "test-session",
		Tmux: "forge-test",
	})
	m.selected = 0

	// handleCancel returns a refresh command
	cmd := m.handleCancel()
	if cmd == nil {
		t.Error("Expected command for cancel")
	}
}

func TestModel_HandleCancel_NonSessionTab(t *testing.T) {
	m := NewModel("/tmp")
	m.activeTab = TabWorkers

	cmd := m.handleCancel()
	if cmd != nil {
		t.Error("Expected nil command for non-session tab")
	}
}

func TestModel_AttachMsg(t *testing.T) {
	m := NewModel("/tmp")
	msg := attachMsg{sessionName: "test-session"}

	newModel, cmd := m.Update(msg)
	model := newModel.(Model)

	if model.attaching != "test-session" {
		t.Errorf("Expected attaching to be 'test-session', got %s", model.attaching)
	}
	if cmd == nil {
		t.Error("Expected doAttach command")
	}
}

func TestModel_SafeWidth(t *testing.T) {
	tests := []struct {
		width    int
		expected int
	}{
		{80, 74},  // 80 - 6 = 74
		{100, 94}, // 100 - 6 = 94
		{46, 40},  // exactly at threshold
		{45, 40},  // below threshold, use minimum
		{30, 40},  // well below threshold
		{0, 40},   // zero width
	}

	for _, tt := range tests {
		m := NewModel("/tmp")
		m.width = tt.width
		got := m.safeWidth()
		if got != tt.expected {
			t.Errorf("safeWidth() with width %d: got %d, want %d", tt.width, got, tt.expected)
		}
	}
}

func TestModel_Init(t *testing.T) {
	m := NewModel("/tmp")
	cmd := m.Init()

	if cmd == nil {
		t.Error("Expected Init to return a command")
	}
}

func TestModel_Update_TickMsg(t *testing.T) {
	m := NewModel("/tmp")
	msg := tickMsg(time.Now())

	_, cmd := m.Update(msg)

	// tick message should return a batch command (refresh + tick)
	if cmd == nil {
		t.Error("Expected command from tick message")
	}
}

func TestModel_Update_SessionRefreshedMsg(t *testing.T) {
	m := NewModel("/tmp")
	m.activeTab = TabSessions

	sessions := []*session.Session{
		{ID: "test", Tmux: "forge-test", State: &ralph.State{Iteration: 5}},
	}
	msg := sessionRefreshedMsg{sessions: sessions}

	_, _ = m.Update(msg)
	// Legacy handler - just verify it doesn't panic
}

func TestComputeWorkerHealth_IdleWorker(t *testing.T) {
	workers := []*worker.Worker{
		{
			ID:         "w-test1234",
			Name:       "alpha",
			Status:     worker.StatusIdle,
			LastActive: time.Now().Add(-5 * time.Minute),
		},
	}
	sessions := []*session.Session{}

	health := computeWorkerHealth(workers, sessions)

	h, ok := health["w-test1234"]
	if !ok {
		t.Fatal("Expected health entry for worker")
	}

	if h.IsStuck {
		t.Error("Idle worker should not be marked as stuck")
	}
	if h.SessionActive {
		t.Error("Idle worker should not have active session")
	}
	if h.PromiseStatus != PromiseNone {
		t.Errorf("Expected PromiseNone, got %v", h.PromiseStatus)
	}
}

func TestComputeWorkerHealth_ActiveWorker(t *testing.T) {
	workers := []*worker.Worker{
		{
			ID:         "w-test5678",
			Name:       "bravo",
			Status:     worker.StatusActive,
			SessionID:  "forge-bravo-test5678",
			LastActive: time.Now().Add(-10 * time.Minute),
		},
	}
	sessions := []*session.Session{}

	health := computeWorkerHealth(workers, sessions)

	h, ok := health["w-test5678"]
	if !ok {
		t.Fatal("Expected health entry for worker")
	}

	// Active worker with recent activity should not be stuck
	if h.IsStuck {
		t.Error("Active worker with recent activity should not be stuck")
	}
}

func TestComputeWorkerHealth_StuckWorker(t *testing.T) {
	workers := []*worker.Worker{
		{
			ID:         "w-stuck123",
			Name:       "charlie",
			Status:     worker.StatusActive,
			SessionID:  "forge-charlie-stuck123",
			LastActive: time.Now().Add(-45 * time.Minute), // >30m = stuck
		},
	}
	sessions := []*session.Session{}

	health := computeWorkerHealth(workers, sessions)

	h, ok := health["w-stuck123"]
	if !ok {
		t.Fatal("Expected health entry for worker")
	}

	if !h.IsStuck {
		t.Error("Worker with >30m inactivity should be marked as stuck")
	}
	if h.StuckDuration < 30*time.Minute {
		t.Errorf("Expected stuck duration >30m, got %v", h.StuckDuration)
	}
}

func TestPromiseStatus_Constants(t *testing.T) {
	// Verify promise status constants are distinct
	if PromiseNone == PromisePending {
		t.Error("PromiseNone and PromisePending should be different")
	}
	if PromisePending == PromiseDetected {
		t.Error("PromisePending and PromiseDetected should be different")
	}
	if PromiseNone == PromiseDetected {
		t.Error("PromiseNone and PromiseDetected should be different")
	}
}

func TestStuckThreshold(t *testing.T) {
	if StuckThreshold != 30*time.Minute {
		t.Errorf("Expected stuck threshold to be 30m, got %v", StuckThreshold)
	}
}

func TestFormatDurationLong(t *testing.T) {
	tests := []struct {
		input time.Duration
		want  string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m 30s"},
		{5 * time.Minute, "5m"},
		{65 * time.Minute, "1h 5m"},
		{2 * time.Hour, "2h"},
		{25 * time.Hour, "1d 1h"},
		{48 * time.Hour, "2d"},
	}

	for _, tt := range tests {
		got := formatDurationLong(tt.input)
		if got != tt.want {
			t.Errorf("formatDurationLong(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
