package worker

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Registry manages the collection of workers.
type Registry struct {
	Workers map[string]*Worker `yaml:"workers"`
	path    string
	mu      sync.RWMutex
}

// RegistryData is the YAML structure for the registry file.
type RegistryData struct {
	Workers map[string]*Worker `yaml:"workers"`
}

// DefaultRegistryPath returns the default path for the worker registry.
func DefaultRegistryPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".forge", "workers", "registry.yaml")
}

// LoadRegistry loads the registry from the default path.
func LoadRegistry() (*Registry, error) {
	return LoadRegistryFrom(DefaultRegistryPath())
}

// LoadRegistryFrom loads the registry from a specific path.
func LoadRegistryFrom(path string) (*Registry, error) {
	r := &Registry{
		Workers: make(map[string]*Worker),
		path:    path,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty registry if file doesn't exist
			return r, nil
		}
		return nil, fmt.Errorf("reading registry: %w", err)
	}

	var regData RegistryData
	if err := yaml.Unmarshal(data, &regData); err != nil {
		return nil, fmt.Errorf("parsing registry: %w", err)
	}

	if regData.Workers != nil {
		r.Workers = regData.Workers
	}

	return r, nil
}

// Save persists the registry to disk.
func (r *Registry) Save() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.saveUnlocked()
}

func (r *Registry) saveUnlocked() error {
	// Ensure directory exists
	dir := filepath.Dir(r.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating registry directory: %w", err)
	}

	data := RegistryData{Workers: r.Workers}
	content, err := yaml.Marshal(&data)
	if err != nil {
		return fmt.Errorf("marshaling registry: %w", err)
	}

	// Write atomically
	tmpPath := r.path + ".tmp"
	if err := os.WriteFile(tmpPath, content, 0644); err != nil {
		return fmt.Errorf("writing registry: %w", err)
	}

	if err := os.Rename(tmpPath, r.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming registry: %w", err)
	}

	return nil
}

// Create creates a new worker with the given role and optional alias.
func (r *Registry) Create(role Role, alias string) (*Worker, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Generate unique ID
	id := GenerateID()

	// Get next available name
	usedNames := make([]string, 0, len(r.Workers))
	for _, w := range r.Workers {
		usedNames = append(usedNames, w.Name)
	}
	name := NextName(usedNames)

	now := time.Now()
	worker := &Worker{
		ID:         id,
		Name:       name,
		Alias:      alias,
		Role:       role,
		Status:     StatusIdle,
		CreatedAt:  now,
		LastActive: now,
	}

	r.Workers[id] = worker

	if err := r.saveUnlocked(); err != nil {
		delete(r.Workers, id)
		return nil, err
	}

	return worker, nil
}

// Get retrieves a worker by ID or name.
func (r *Registry) Get(idOrName string) *Worker {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.getUnlocked(idOrName)
}

func (r *Registry) getUnlocked(idOrName string) *Worker {
	// Try direct ID lookup first
	if w, ok := r.Workers[idOrName]; ok {
		return w
	}

	// Try name or alias match
	idOrName = strings.ToLower(idOrName)
	for _, w := range r.Workers {
		if strings.ToLower(w.Name) == idOrName || strings.ToLower(w.Alias) == idOrName {
			return w
		}
	}

	// Try short ID match
	for _, w := range r.Workers {
		if strings.HasPrefix(w.ID, "w-"+idOrName) || strings.HasPrefix(w.ID[2:], idOrName) {
			return w
		}
	}

	return nil
}

// List returns all workers matching the given status filters.
// If no filters are provided, returns all workers.
func (r *Registry) List(statuses ...Status) []*Worker {
	r.mu.RLock()
	defer r.mu.RUnlock()

	statusFilter := make(map[Status]bool)
	for _, s := range statuses {
		statusFilter[s] = true
	}

	var workers []*Worker
	for _, w := range r.Workers {
		if len(statusFilter) == 0 || statusFilter[w.Status] {
			workers = append(workers, w.Clone())
		}
	}

	// Sort by name for consistent ordering
	sort.Slice(workers, func(i, j int) bool {
		return workers[i].Name < workers[j].Name
	})

	return workers
}

// Update applies a function to update a worker and saves the registry.
func (r *Registry) Update(idOrName string, fn func(*Worker)) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	w := r.getUnlocked(idOrName)
	if w == nil {
		return fmt.Errorf("worker not found: %s", idOrName)
	}

	fn(w)
	w.LastActive = time.Now()

	return r.saveUnlocked()
}

// Delete removes a worker from the registry.
func (r *Registry) Delete(idOrName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	w := r.getUnlocked(idOrName)
	if w == nil {
		return fmt.Errorf("worker not found: %s", idOrName)
	}

	if w.Status == StatusActive {
		return fmt.Errorf("cannot delete active worker: %s", w.Name)
	}

	delete(r.Workers, w.ID)
	return r.saveUnlocked()
}

// FindAvailable returns an available worker with the given role, or nil if none.
func (r *Registry) FindAvailable(role Role) *Worker {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, w := range r.Workers {
		if w.Role == role && w.Status.IsAvailable() {
			return w.Clone()
		}
	}
	return nil
}

// FindByWorktree returns the worker assigned to a worktree, or nil if none.
func (r *Registry) FindByWorktree(worktree string) *Worker {
	r.mu.RLock()
	defer r.mu.RUnlock()

	worktree = filepath.Clean(worktree)
	for _, w := range r.Workers {
		if filepath.Clean(w.Worktree) == worktree {
			return w.Clone()
		}
	}
	return nil
}

// FindBySession returns the worker with the given tmux session ID, or nil if none.
func (r *Registry) FindBySession(sessionID string) *Worker {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, w := range r.Workers {
		if w.SessionID == sessionID {
			return w.Clone()
		}
	}
	return nil
}

// CountByStatus returns the count of workers in each status.
func (r *Registry) CountByStatus() map[Status]int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	counts := make(map[Status]int)
	for _, w := range r.Workers {
		counts[w.Status]++
	}
	return counts
}

// CountByRole returns the count of workers in each role.
func (r *Registry) CountByRole() map[Role]int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	counts := make(map[Role]int)
	for _, w := range r.Workers {
		counts[w.Role]++
	}
	return counts
}

// Path returns the registry file path.
func (r *Registry) Path() string {
	return r.path
}

// Count returns the total number of workers.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.Workers)
}
