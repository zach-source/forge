package session

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/zach-source/forge/internal/ralph"
	"github.com/zach-source/forge/internal/tmux"
)

// Manager handles discovery and tracking of multiple forge sessions.
type Manager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewManager creates a new session manager.
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
	}
}

// Discover finds all active forge sessions from tmux and state files.
func (m *Manager) Discover() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear existing
	m.sessions = make(map[string]*Session)

	// 1. Find all forge-* tmux sessions
	tmuxSessions, err := tmux.ListForgeSessions()
	if err != nil {
		return err
	}

	for _, name := range tmuxSessions {
		m.sessions[name] = &Session{
			ID:     name,
			Tmux:   name,
			Status: StatusActive,
		}
	}

	// 2. Find all state files in ~/.forge/sessions/
	stateFiles, err := ralph.ListSessionStateFiles()
	if err != nil {
		// Non-fatal, continue with what we have
		stateFiles = nil
	}

	for _, path := range stateFiles {
		ctrl := ralph.NewStateController(path)
		state, err := ctrl.Read()
		if err != nil {
			continue
		}

		// Extract session ID from filename
		id := strings.TrimSuffix(filepath.Base(path), ".state.md")

		if existing, ok := m.sessions[id]; ok {
			// Merge with tmux-discovered session
			existing.State = state
			existing.WorkDir = state.WorkDir
			existing.LogFile = state.LogFile
			existing.StartedAt = state.StartedAt
			if !state.Active {
				existing.Status = StatusCompleted
			}
		} else {
			// New session from state file
			status := StatusActive
			if !state.Active {
				status = StatusCompleted
			}
			m.sessions[id] = &Session{
				ID:        id,
				State:     state,
				Tmux:      state.TmuxSession,
				WorkDir:   state.WorkDir,
				LogFile:   state.LogFile,
				Status:    status,
				StartedAt: state.StartedAt,
			}
		}
	}

	// 3. Check for local .claude/ralph-loop.local.md files
	// This is done per-project, not globally

	return nil
}

// DiscoverLocal discovers sessions from the current working directory.
func (m *Manager) DiscoverLocal(workDir string) error {
	localStatePath := filepath.Join(workDir, ".claude", "ralph-loop.local.md")
	if _, err := os.Stat(localStatePath); err == nil {
		ctrl := ralph.NewStateController(localStatePath)
		state, err := ctrl.Read()
		if err == nil {
			m.mu.Lock()
			defer m.mu.Unlock()

			id := state.ID
			if id == "" {
				id = "local"
			}

			status := StatusActive
			if !state.Active {
				status = StatusCompleted
			}

			m.sessions[id] = &Session{
				ID:        id,
				State:     state,
				Tmux:      state.TmuxSession,
				WorkDir:   workDir,
				LogFile:   state.LogFile,
				Status:    status,
				StartedAt: state.StartedAt,
			}
		}
	}
	return nil
}

// List returns all known sessions.
func (m *Manager) List() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// Get returns a specific session by ID.
func (m *Manager) Get(id string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[id]
}

// Add adds a session to the manager.
func (m *Manager) Add(session *Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[session.ID] = session
}

// Remove removes a session from the manager.
func (m *Manager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
}

// Refresh updates the state of all sessions.
func (m *Manager) Refresh() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, session := range m.sessions {
		// Check if tmux session still exists
		if session.Tmux != "" {
			tmuxSession := tmux.NewSession(session.Tmux, "", "")
			if !tmuxSession.Exists() {
				session.Status = StatusCompleted
			}
		}

		// Re-read state file if we have a path
		if session.State != nil && session.LogFile != "" {
			statePath := ralph.SessionStatePath(id)
			if _, err := os.Stat(statePath); err == nil {
				ctrl := ralph.NewStateController(statePath)
				if state, err := ctrl.Read(); err == nil {
					session.State = state
					if !state.Active {
						session.Status = StatusCompleted
					}
				}
			}
		}
	}

	return nil
}

// ActiveCount returns the number of active sessions.
func (m *Manager) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, s := range m.sessions {
		if s.Status == StatusActive {
			count++
		}
	}
	return count
}

// CaptureOutput captures the last n lines of output from a session.
func (m *Manager) CaptureOutput(id string, lines int) ([]string, error) {
	m.mu.RLock()
	session := m.sessions[id]
	m.mu.RUnlock()

	if session == nil {
		return nil, nil
	}

	if session.Tmux == "" {
		return nil, nil
	}

	tmuxSession := tmux.NewSession(session.Tmux, "", "")
	return tmuxSession.CapturePaneLines(lines)
}
