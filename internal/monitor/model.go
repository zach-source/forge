package monitor

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/zach-source/forge/internal/ralph"
	"github.com/zach-source/forge/internal/session"
	"github.com/zach-source/forge/internal/tmux"
)

// tickMsg is sent on each refresh tick.
type tickMsg time.Time

// sessionRefreshedMsg is sent after sessions are refreshed.
type sessionRefreshedMsg struct {
	sessions []*session.Session
}

// attachMsg is sent when we should attach to a session.
type attachMsg struct {
	sessionName string
}

// Model is the Bubbletea model for the monitor TUI.
type Model struct {
	sessions  *session.Manager
	selected  int
	output    []string
	width     int
	height    int
	keyMap    KeyMap
	workDir   string
	quitting  bool
	attaching string
	error     string
}

// NewModel creates a new monitor model.
func NewModel(workDir string) Model {
	mgr := session.NewManager()
	return Model{
		sessions: mgr,
		keyMap:   DefaultKeyMap(),
		workDir:  workDir,
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.refreshSessions,
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
		return m, tea.Batch(m.refreshSessions, m.tick())

	case sessionRefreshedMsg:
		// Update output for selected session
		sessions := msg.sessions
		if len(sessions) > 0 && m.selected < len(sessions) {
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

		case key.Matches(msg, m.keyMap.Up):
			if m.selected > 0 {
				m.selected--
				m.updateOutput()
			}
			return m, nil

		case key.Matches(msg, m.keyMap.Down):
			sessions := m.sessions.List()
			if m.selected < len(sessions)-1 {
				m.selected++
				m.updateOutput()
			}
			return m, nil

		case key.Matches(msg, m.keyMap.Attach):
			sessions := m.sessions.List()
			if len(sessions) > 0 && m.selected < len(sessions) {
				s := sessions[m.selected]
				if s.Tmux != "" {
					return m, func() tea.Msg {
						return attachMsg{sessionName: s.Tmux}
					}
				}
			}
			return m, nil

		case key.Matches(msg, m.keyMap.Cancel):
			sessions := m.sessions.List()
			if len(sessions) > 0 && m.selected < len(sessions) {
				s := sessions[m.selected]
				m.cancelSession(s.ID)
			}
			return m, m.refreshSessions

		case key.Matches(msg, m.keyMap.Refresh):
			return m, m.refreshSessions
		}
	}

	return m, nil
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

// refreshSessions fetches the latest session list.
func (m Model) refreshSessions() tea.Msg {
	m.sessions.Discover()
	m.sessions.DiscoverLocal(m.workDir)
	return sessionRefreshedMsg{sessions: m.sessions.List()}
}

// updateOutput updates the output preview for the selected session.
func (m *Model) updateOutput() {
	sessions := m.sessions.List()
	if len(sessions) > 0 && m.selected < len(sessions) {
		s := sessions[m.selected]
		if output, err := m.sessions.CaptureOutput(s.ID, 15); err == nil {
			m.output = output
		} else {
			m.output = nil
		}
	}
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
	return tea.ExecProcess(exec.Command("tmux", "attach", "-t", sessionName), func(err error) tea.Msg {
		return nil
	})
}
