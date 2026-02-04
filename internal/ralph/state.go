// Package ralph provides state management for the Ralph Loop execution.
package ralph

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// State represents the current state of a Ralph Loop execution.
type State struct {
	// ID is the unique identifier for this session
	ID string `yaml:"id"`
	// Active indicates if the loop is currently running
	Active bool `yaml:"active"`
	// Iteration is the current iteration count
	Iteration int `yaml:"iteration"`
	// MaxIterations is the maximum number of iterations (0 = unlimited)
	MaxIterations int `yaml:"max_iterations"`
	// CompletionPromise is the text that indicates completion
	CompletionPromise string `yaml:"completion_promise"`
	// StartedAt is when the loop was started
	StartedAt time.Time `yaml:"started_at"`
	// TmuxSession is the name of the tmux session running this loop
	TmuxSession string `yaml:"tmux_session"`
	// WorkDir is the working directory for the session
	WorkDir string `yaml:"workdir"`
	// LogFile is the path to the output log
	LogFile string `yaml:"log_file"`
	// Prompt is the original prompt (body content after frontmatter)
	Prompt string `yaml:"-"`
}

// Status represents the current status of a session
type Status string

const (
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
	StatusCancelled Status = "cancelled"
	StatusError     Status = "error"
	StatusPaused    Status = "paused"
)

var (
	ErrStateNotFound = errors.New("state file not found")
	ErrInvalidState  = errors.New("invalid state file format")
)

// StateController manages reading and writing state files.
type StateController struct {
	path string
	mu   sync.RWMutex
}

// NewStateController creates a new state controller for the given path.
func NewStateController(path string) *StateController {
	return &StateController{path: path}
}

// Path returns the path to the state file.
func (c *StateController) Path() string {
	return c.path
}

// Exists returns true if the state file exists.
func (c *StateController) Exists() bool {
	_, err := os.Stat(c.path)
	return err == nil
}

// Read reads the state from the file.
func (c *StateController) Read() (*State, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, err := os.ReadFile(c.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrStateNotFound
		}
		return nil, fmt.Errorf("reading state file: %w", err)
	}

	return ParseState(string(data))
}

// Write writes the state to the file atomically.
func (c *StateController) Write(state *State) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	content, err := FormatState(state)
	if err != nil {
		return fmt.Errorf("formatting state: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	// Write atomically via temp file
	tmpPath := c.path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing temp state file: %w", err)
	}

	if err := os.Rename(tmpPath, c.path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming state file: %w", err)
	}

	return nil
}

// Update reads the current state, applies the updater function, and writes back.
func (c *StateController) Update(updater func(*State)) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(c.path)
	if err != nil {
		return fmt.Errorf("reading state file: %w", err)
	}

	state, err := ParseState(string(data))
	if err != nil {
		return fmt.Errorf("parsing state: %w", err)
	}

	updater(state)

	content, err := FormatState(state)
	if err != nil {
		return fmt.Errorf("formatting state: %w", err)
	}

	tmpPath := c.path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing temp state file: %w", err)
	}

	if err := os.Rename(tmpPath, c.path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming state file: %w", err)
	}

	return nil
}

// Delete removes the state file.
func (c *StateController) Delete() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := os.Remove(c.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing state file: %w", err)
	}
	return nil
}

// ParseState parses a state file with YAML frontmatter.
func ParseState(content string) (*State, error) {
	content = strings.TrimSpace(content)

	// Check for frontmatter
	if !strings.HasPrefix(content, "---") {
		return nil, ErrInvalidState
	}

	// Find the end of frontmatter
	rest := content[3:]
	endIdx := strings.Index(rest, "\n---")
	if endIdx == -1 {
		return nil, ErrInvalidState
	}

	frontmatter := strings.TrimSpace(rest[:endIdx])
	body := strings.TrimSpace(rest[endIdx+4:])

	var state State
	if err := yaml.Unmarshal([]byte(frontmatter), &state); err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}

	state.Prompt = body
	return &state, nil
}

// FormatState formats a state struct into a markdown file with YAML frontmatter.
func FormatState(state *State) (string, error) {
	frontmatter, err := yaml.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("marshaling frontmatter: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(frontmatter)
	sb.WriteString("---\n\n")
	sb.WriteString(state.Prompt)
	sb.WriteString("\n")

	return sb.String(), nil
}

// DefaultSessionsDir returns the default directory for session state files.
func DefaultSessionsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".forge", "sessions")
}

// SessionStatePath returns the path to a session's state file.
func SessionStatePath(id string) string {
	return filepath.Join(DefaultSessionsDir(), id+".state.md")
}

// ListSessionStateFiles returns all session state files.
func ListSessionStateFiles() ([]string, error) {
	dir := DefaultSessionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".state.md") {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	return files, nil
}
