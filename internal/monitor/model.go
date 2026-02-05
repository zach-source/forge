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
	workerHealth  map[string]*WorkerHealth
	kanban        *kanban.Board
	logEntries    []logs.LogEntry
}

// WorkerHealth contains health metrics for a worker.
type WorkerHealth struct {
	// Uptime is how long the worker has been running (if active)
	Uptime time.Duration
	// LastActivity is when the worker last had activity
	LastActivity time.Time
	// IsStuck indicates the worker has had no activity for >30m
	IsStuck bool
	// StuckDuration is how long the worker has been stuck
	StuckDuration time.Duration
	// PromiseStatus indicates promise detection state
	PromiseStatus PromiseStatus
	// Promise is the expected completion promise text
	Promise string
	// Iteration is the current iteration count
	Iteration int
	// MaxIterations is the max iterations (0 = unlimited)
	MaxIterations int
	// SessionActive indicates if the tmux session is still running
	SessionActive bool
}

// PromiseStatus indicates the state of promise detection.
type PromiseStatus int

const (
	// PromiseNone means no promise is configured
	PromiseNone PromiseStatus = iota
	// PromisePending means waiting for promise to be detected
	PromisePending
	// PromiseDetected means the promise was found in output
	PromiseDetected
)

// StuckThreshold is the duration after which a worker is considered stuck.
const StuckThreshold = 30 * time.Minute

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
	workerHealth  map[string]*WorkerHealth
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
		m.workerHealth = msg.workerHealth
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
	_ = m.sessions.Discover()
	_ = m.sessions.DiscoverLocal(m.workDir)
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

	// Compute worker health
	workerHealth := computeWorkerHealth(workers, sessions)
	// Refresh kanban
	var board *kanban.Board
	if store, err := kanban.NewStore(m.workDir); err == nil {
		board, _ = store.GetBoard()
		_ = store.Close()
	}

	// Refresh log entries
	logEntries, _ := logs.ListLogs(logs.ListOptions{})

	return dataRefreshedMsg{
		sessions:      sessions,
		workers:       workers,
		workerMetrics: workerMetrics,
		workerHealth:  workerHealth,
		kanban:        board,
		logEntries:    logEntries,
	}
}

// computeWorkerHealth computes health metrics for all workers.
func computeWorkerHealth(workers []*worker.Worker, sessions []*session.Session) map[string]*WorkerHealth {
	health := make(map[string]*WorkerHealth)

	// Build session lookup by ID
	sessionByID := make(map[string]*session.Session)
	for _, s := range sessions {
		sessionByID[s.ID] = s
	}

	for _, w := range workers {
		h := &WorkerHealth{
			LastActivity:  w.LastActive,
			PromiseStatus: PromiseNone,
		}

		// Calculate uptime for active workers
		if w.Status == worker.StatusActive && !w.LastActive.IsZero() {
			h.Uptime = time.Since(w.LastActive)
		}

		// Check if stuck (no activity for >30m and worker is active)
		if w.Status == worker.StatusActive && !w.LastActive.IsZero() {
			timeSinceActive := time.Since(w.LastActive)
			if timeSinceActive > StuckThreshold {
				h.IsStuck = true
				h.StuckDuration = timeSinceActive
			}
		}

		// Get session info if worker has an active session
		if w.SessionID != "" {
			// Try to find session by tmux session name
			tmuxName := w.TmuxSessionName()
			for _, s := range sessions {
				if s.Tmux == tmuxName || s.ID == w.SessionID {
					h.SessionActive = s.Status == session.StatusActive

					// Get state info from session
					if s.State != nil {
						h.Iteration = s.State.Iteration
						h.MaxIterations = s.State.MaxIterations
						h.Promise = s.State.CompletionPromise

						if h.Promise != "" {
							h.PromiseStatus = PromisePending
						}

						// Update uptime from session start time
						if !s.State.StartedAt.IsZero() {
							h.Uptime = time.Since(s.State.StartedAt)
						}

						// Check if session is still active in state
						if s.State.Active {
							h.SessionActive = true
						}
					}
					break
				}
			}

			// Also try reading state file directly for more accurate info
			statePath := ralph.SessionStatePath(w.SessionID)
			ctrl := ralph.NewStateController(statePath)
			if state, err := ctrl.Read(); err == nil {
				h.Iteration = state.Iteration
				h.MaxIterations = state.MaxIterations
				h.Promise = state.CompletionPromise
				h.SessionActive = state.Active

				if h.Promise != "" {
					h.PromiseStatus = PromisePending
				}

				if !state.StartedAt.IsZero() {
					h.Uptime = time.Since(state.StartedAt)
				}
			}

			// Check tmux session exists
			tmuxSession := tmux.NewSession(tmuxName, "", "")
			if tmuxSession.Exists() {
				h.SessionActive = true
			}
		}

		health[w.ID] = h
	}

	return health
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
	defer func() { _ = file.Close() }()

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
	_ = os.Remove(statePath)

	// Kill tmux session
	tmuxSession := tmux.NewSession(id, "", "")
	_ = tmuxSession.Kill()
}

// doAttach returns a command that attaches to a tmux session.
func (m Model) doAttach(sessionName string) tea.Cmd {
	return tea.ExecProcess(tmux.AttachCmd(sessionName), func(err error) tea.Msg {
		return nil
	})
}
