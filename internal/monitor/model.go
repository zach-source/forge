package monitor

import (
	"bufio"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/zach-source/forge/internal/kanban"
	"github.com/zach-source/forge/internal/logs"
	"github.com/zach-source/forge/internal/ralph"
	"github.com/zach-source/forge/internal/session"
	"github.com/zach-source/forge/internal/tmux"
	"github.com/zach-source/forge/internal/worker"
)

// Tab represents a tab in the monitor TUI.
type Tab int

const (
	TabOverview Tab = iota
	TabWorkers
	TabSessions
	TabLogs
)

// tabNames returns the display names for tabs.
func tabNames() []string {
	return []string{"Overview", "Workers", "Sessions", "Logs"}
}

// tickMsg is sent on each refresh tick.
type tickMsg time.Time

// sessionRefreshedMsg is sent after sessions are refreshed.
type sessionRefreshedMsg struct {
	sessions []*session.Session
}

// dataRefreshedMsg is sent after all data is refreshed.
type dataRefreshedMsg struct {
	sessions      []*session.Session
	workers       []*worker.Worker
	workerMetrics map[string]*worker.WorkerMetrics
	kanban        *kanban.Board
	logEntries    []logs.LogEntry
}

// attachMsg is sent when we should attach to a session.
type attachMsg struct {
	sessionName string
}

// Model is the Bubbletea model for the monitor TUI.
type Model struct {
	// Tab state
	activeTab Tab
	selected  int // selected item in current tab

	// Data
	sessions      *session.Manager
	workers       []*worker.Worker
	workerMetrics map[string]*worker.WorkerMetrics
	kanban        *kanban.Board
	logEntries    []logs.LogEntry

	// Output preview
	output []string

	// Dimensions
	width  int
	height int

	// Config
	keyMap  KeyMap
	workDir string

	// State
	quitting  bool
	attaching string
	error     string
}

// NewModel creates a new monitor model.
func NewModel(workDir string) Model {
	mgr := session.NewManager()
	return Model{
		activeTab: TabOverview,
		sessions:  mgr,
		keyMap:    DefaultKeyMap(),
		workDir:   workDir,
		width:     80, // Default before WindowSizeMsg
		height:    24, // Default before WindowSizeMsg
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.refreshAll,
		m.tick(),
	)
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		return m, tea.Batch(m.refreshAll, m.tick())

	case dataRefreshedMsg:
		m.workers = msg.workers
		m.workerMetrics = msg.workerMetrics
		m.kanban = msg.kanban
		m.logEntries = msg.logEntries
		// Update output for selected session
		sessions := msg.sessions
		if len(sessions) > 0 && m.activeTab == TabSessions && m.selected < len(sessions) {
			s := sessions[m.selected]
			if output, err := m.sessions.CaptureOutput(s.ID, 15); err == nil {
				m.output = output
			}
		}
		return m, nil

	case sessionRefreshedMsg:
		// Legacy handler - update output for selected session
		sessions := msg.sessions
		if len(sessions) > 0 && m.activeTab == TabSessions && m.selected < len(sessions) {
			s := sessions[m.selected]
			if output, err := m.sessions.CaptureOutput(s.ID, 15); err == nil {
				m.output = output
			}
		}
		return m, nil

	case attachMsg:
		// Launch tmux attach in a new process
		m.attaching = msg.sessionName
		return m, m.doAttach(msg.sessionName)

	case tea.KeyMsg:
		// Clear any error on keypress
		m.error = ""

		switch {
		case key.Matches(msg, m.keyMap.Quit):
			m.quitting = true
			return m, tea.Quit

		case key.Matches(msg, m.keyMap.TabNext):
			m.activeTab = (m.activeTab + 1) % 4
			m.selected = 0
			m.output = nil
			return m, nil

		case key.Matches(msg, m.keyMap.TabPrev):
			m.activeTab = (m.activeTab + 3) % 4 // +3 is same as -1 mod 4
			m.selected = 0
			m.output = nil
			return m, nil

		case key.Matches(msg, m.keyMap.Tab1):
			m.activeTab = TabOverview
			m.selected = 0
			return m, nil

		case key.Matches(msg, m.keyMap.Tab2):
			m.activeTab = TabWorkers
			m.selected = 0
			return m, nil

		case key.Matches(msg, m.keyMap.Tab3):
			m.activeTab = TabSessions
			m.selected = 0
			return m, nil

		case key.Matches(msg, m.keyMap.Tab4):
			m.activeTab = TabLogs
			m.selected = 0
			return m, nil

		case key.Matches(msg, m.keyMap.Up):
			if m.selected > 0 {
				m.selected--
				m.updateOutput()
			}
			return m, nil

		case key.Matches(msg, m.keyMap.Down):
			maxItems := m.maxItemsForTab()
			if m.selected < maxItems-1 {
				m.selected++
				m.updateOutput()
			}
			return m, nil

		case key.Matches(msg, m.keyMap.Attach):
			return m, m.handleAttach()

		case key.Matches(msg, m.keyMap.Cancel):
			return m, m.handleCancel()

		case key.Matches(msg, m.keyMap.Refresh):
			return m, m.refreshAll
		}
	}

	return m, nil
}

// maxItemsForTab returns the max items for the current tab.
func (m Model) maxItemsForTab() int {
	switch m.activeTab {
	case TabWorkers:
		return len(m.workers)
	case TabSessions:
		return len(m.sessions.List())
	case TabLogs:
		return len(m.logEntries)
	default:
		return 0
	}
}

// handleAttach handles the attach action based on current tab.
func (m Model) handleAttach() tea.Cmd {
	switch m.activeTab {
	case TabSessions:
		sessions := m.sessions.List()
		if len(sessions) > 0 && m.selected < len(sessions) {
			s := sessions[m.selected]
			if s.Tmux != "" {
				return func() tea.Msg {
					return attachMsg{sessionName: s.Tmux}
				}
			}
		}
	case TabWorkers:
		if len(m.workers) > 0 && m.selected < len(m.workers) {
			w := m.workers[m.selected]
			if w.SessionID != "" {
				return func() tea.Msg {
					return attachMsg{sessionName: w.TmuxSessionName()}
				}
			}
		}
	}
	return nil
}

// handleCancel handles the cancel action based on current tab.
func (m *Model) handleCancel() tea.Cmd {
	switch m.activeTab {
	case TabSessions:
		sessions := m.sessions.List()
		if len(sessions) > 0 && m.selected < len(sessions) {
			s := sessions[m.selected]
			m.cancelSession(s.ID)
		}
		return m.refreshAll
	}
	return nil
}

// View renders the TUI.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	if m.attaching != "" {
		return fmt.Sprintf("Attaching to %s...\n", m.attaching)
	}

	return m.renderView()
}

// tick returns a command that sends a tick message.
func (m Model) tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// refreshAll fetches all data.
func (m Model) refreshAll() tea.Msg {
	// Refresh sessions
	m.sessions.Discover()
	m.sessions.DiscoverLocal(m.workDir)
	sessions := m.sessions.List()

	// Refresh workers
	var workers []*worker.Worker
	if reg, err := worker.LoadRegistry(); err == nil {
		workers = reg.List()
	}

	// Refresh worker metrics
	var workerMetrics map[string]*worker.WorkerMetrics
	if metricsStore, err := worker.NewMetricsStore(); err == nil {
		workerMetrics = metricsStore.GetAllWorkerMetrics()
	}

	// Refresh kanban
	var board *kanban.Board
	if store, err := kanban.NewStore(m.workDir); err == nil {
		board, _ = store.GetBoard()
		store.Close()
	}

	// Refresh log entries
	logEntries, _ := logs.ListLogs(logs.ListOptions{})

	return dataRefreshedMsg{
		sessions:      sessions,
		workers:       workers,
		workerMetrics: workerMetrics,
		kanban:        board,
		logEntries:    logEntries,
	}
}

// updateOutput updates the output preview for the selected item.
func (m *Model) updateOutput() {
	switch m.activeTab {
	case TabSessions:
		sessions := m.sessions.List()
		if len(sessions) > 0 && m.selected < len(sessions) {
			s := sessions[m.selected]
			// Try tmux capture first
			if output, err := m.sessions.CaptureOutput(s.ID, 30); err == nil && len(output) > 0 {
				m.output = output
				return
			}
			// Fall back to log file
			if s.LogFile != "" {
				m.output = readLastLines(s.LogFile, 30)
			} else {
				m.output = nil
			}
		}
	case TabWorkers:
		if len(m.workers) > 0 && m.selected < len(m.workers) {
			w := m.workers[m.selected]
			// Try to get log from worker's log file
			logPath := logs.WorkerLogPath(w.Name)
			m.output = readLastLines(logPath, 30)
		}
	default:
		m.output = nil
	}
}

// readLastLines reads the last n lines from a file.
func readLastLines(path string, n int) []string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		// Keep buffer at 2x size to avoid constant reallocation
		if len(lines) > n*2 {
			lines = lines[len(lines)-n:]
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

// cancelSession cancels a session.
func (m *Model) cancelSession(id string) {
	// Delete state file
	statePath := ralph.SessionStatePath(id)
	os.Remove(statePath)

	// Kill tmux session
	tmuxSession := tmux.NewSession(id, "", "")
	tmuxSession.Kill()
}

// doAttach returns a command that attaches to a tmux session.
func (m Model) doAttach(sessionName string) tea.Cmd {
	return tea.ExecProcess(tmux.AttachCmd(sessionName), func(err error) tea.Msg {
		return nil
	})
}
