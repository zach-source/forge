package dashboard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/zach-source/forge/internal/kanban"
	"github.com/zach-source/forge/internal/worker"
)

// Tab represents a tab in the dashboard.
type Tab int

const (
	TabOverview Tab = iota
	TabWorkers
	TabLeaders
	TabTasks
	TabLogs
)

// tabNames returns the display names for tabs.
func tabNames() []string {
	return []string{"Overview", "Workers", "Leaders", "Tasks", "Logs"}
}

// tickMsg is sent on each refresh tick.
type tickMsg time.Time

// dataRefreshedMsg is sent after all data is refreshed.
type dataRefreshedMsg struct {
	workers         []*worker.Worker
	board           *kanban.Board
	health          *HealthStatus
	supervisorState *SupervisorStateSnapshot
}

// HealthStatus represents infrastructure health from .forge/health/status.json.
type HealthStatus struct {
	Timestamp string `json:"timestamp"`
	Status    string `json:"status"`
	Cluster   struct {
		NodesReady  int `json:"nodes_ready"`
		NodesTotal  int `json:"nodes_total"`
		PodsRunning int `json:"pods_running"`
		PodsFailed  int `json:"pods_failed"`
	} `json:"cluster"`
	Alerts struct {
		Critical int `json:"critical"`
		Warning  int `json:"warning"`
		Info     int `json:"info"`
	} `json:"alerts"`
}

// SupervisorStateSnapshot is a snapshot of the supervisor's persisted state.
type SupervisorStateSnapshot struct {
	StartedAt  time.Time              `json:"started_at"`
	LastCycle  time.Time              `json:"last_cycle"`
	CycleCount int                    `json:"cycle_count"`
	Leaders    map[string]LeaderState `json:"leaders"`
}

// LeaderState is the state of a single leader.
type LeaderState struct {
	Running   bool      `json:"running"`
	SessionID string    `json:"session_id"`
	StartedAt time.Time `json:"started_at"`
}

// Model is the Bubbletea model for the supervisor dashboard.
type Model struct {
	// Tab state
	activeTab Tab
	selected  int

	// Data
	workers         []*worker.Worker
	board           *kanban.Board
	health          *HealthStatus
	supervisorState *SupervisorStateSnapshot

	// Dimensions
	width  int
	height int

	// Config
	keyMap  KeyMap
	workDir string

	// State
	quitting bool
	error    string
}

// NewModel creates a new dashboard model.
func NewModel(workDir string) Model {
	return Model{
		activeTab: TabOverview,
		keyMap:    DefaultKeyMap(),
		workDir:   workDir,
		width:     120,
		height:    30,
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
		m.board = msg.board
		m.health = msg.health
		m.supervisorState = msg.supervisorState
		return m, nil

	case tea.KeyMsg:
		m.error = ""

		switch {
		case key.Matches(msg, m.keyMap.Quit):
			m.quitting = true
			return m, tea.Quit

		case key.Matches(msg, m.keyMap.TabNext):
			m.activeTab = (m.activeTab + 1) % 5
			m.selected = 0
			return m, nil

		case key.Matches(msg, m.keyMap.TabPrev):
			m.activeTab = (m.activeTab + 4) % 5
			m.selected = 0
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
			m.activeTab = TabLeaders
			m.selected = 0
			return m, nil

		case key.Matches(msg, m.keyMap.Tab4):
			m.activeTab = TabTasks
			m.selected = 0
			return m, nil

		case key.Matches(msg, m.keyMap.Tab5):
			m.activeTab = TabLogs
			m.selected = 0
			return m, nil

		case key.Matches(msg, m.keyMap.Up):
			if m.selected > 0 {
				m.selected--
			}
			return m, nil

		case key.Matches(msg, m.keyMap.Down):
			maxItems := m.maxItemsForTab()
			if m.selected < maxItems-1 {
				m.selected++
			}
			return m, nil

		case key.Matches(msg, m.keyMap.Refresh):
			return m, m.refreshAll
		}
	}

	return m, nil
}

// tick returns a command that sends a tick message after 2 seconds.
func (m Model) tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// refreshAll refreshes all data sources.
func (m Model) refreshAll() tea.Msg {
	msg := dataRefreshedMsg{}

	// Load workers
	if reg, err := worker.LoadRegistry(); err == nil {
		msg.workers = reg.List()
	}

	// Load kanban board
	beadsDir := filepath.Join(m.workDir, ".beads")
	if store, err := kanban.NewStore(beadsDir); err == nil {
		defer func() { _ = store.Close() }()
		if board, err := store.GetBoard(); err == nil {
			msg.board = board
		}
	}

	// Load health status
	healthPath := filepath.Join(m.workDir, ".forge", "health", "status.json")
	if data, err := os.ReadFile(healthPath); err == nil {
		var health HealthStatus
		if json.Unmarshal(data, &health) == nil {
			msg.health = &health
		}
	}

	// Load supervisor state
	statePath := filepath.Join(m.workDir, ".forge", "supervisor", "state.json")
	if data, err := os.ReadFile(statePath); err == nil {
		var ss SupervisorStateSnapshot
		if json.Unmarshal(data, &ss) == nil {
			msg.supervisorState = &ss
		}
	}

	return msg
}

// maxItemsForTab returns the maximum number of selectable items for the current tab.
func (m Model) maxItemsForTab() int {
	switch m.activeTab {
	case TabWorkers:
		return len(m.workers)
	case TabLeaders:
		if m.supervisorState != nil {
			return len(m.supervisorState.Leaders)
		}
		return 0
	case TabTasks:
		if m.board != nil {
			total := 0
			for _, col := range m.board.Columns {
				total += len(col.Issues)
			}
			return total
		}
		return 0
	default:
		return 0
	}
}
