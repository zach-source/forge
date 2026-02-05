// Package context provides automatic context injection for workers.
package context

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// MaxLearnings is the maximum number of learnings to retain.
const MaxLearnings = 50

// DefaultLearningsFile is the default filename for learnings storage.
const DefaultLearningsFile = "learnings.json"

// Learning represents a captured failure-fix pattern.
type Learning struct {
	ID         string    `json:"id"`
	Summary    string    `json:"summary"`  // One-line summary
	Problem    string    `json:"problem"`  // What went wrong
	Solution   string    `json:"solution"` // How it was fixed
	Keywords   []string  `json:"keywords"` // Matching keywords
	Files      []string  `json:"files"`    // Related files
	CreatedAt  time.Time `json:"created_at"`
	SourceTask string    `json:"source_task"` // Where this came from
}

// LearningsStore holds the collection of learnings.
type LearningsStore struct {
	Learnings []Learning `json:"learnings"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// NewStore creates an empty learnings store.
func NewStore() *LearningsStore {
	return &LearningsStore{
		Learnings: []Learning{},
		UpdatedAt: time.Now(),
	}
}

// LoadStore loads learnings from the default location in the given workspace.
func LoadStore(workspaceDir string) (*LearningsStore, error) {
	path := LearningsPath(workspaceDir)
	return LoadStoreFromPath(path)
}

// LoadStoreFromPath loads learnings from a specific file path.
func LoadStoreFromPath(path string) (*LearningsStore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewStore(), nil
		}
		return nil, err
	}

	var store LearningsStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, err
	}

	return &store, nil
}

// Save writes the learnings store to the default location.
func (s *LearningsStore) Save(workspaceDir string) error {
	path := LearningsPath(workspaceDir)
	return s.SaveToPath(path)
}

// SaveToPath writes the learnings store to a specific file path.
func (s *LearningsStore) SaveToPath(path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	s.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// Add adds a new learning to the store, maintaining the max size.
func (s *LearningsStore) Add(learning Learning) {
	// Assign ID if not set
	if learning.ID == "" {
		learning.ID = generateLearningID()
	}

	// Set creation time if not set
	if learning.CreatedAt.IsZero() {
		learning.CreatedAt = time.Now()
	}

	// Prepend to maintain newest-first order
	s.Learnings = append([]Learning{learning}, s.Learnings...)

	// Trim to max size
	if len(s.Learnings) > MaxLearnings {
		s.Learnings = s.Learnings[:MaxLearnings]
	}

	s.UpdatedAt = time.Now()
}

// GetByID finds a learning by its ID.
func (s *LearningsStore) GetByID(id string) *Learning {
	for i := range s.Learnings {
		if s.Learnings[i].ID == id {
			return &s.Learnings[i]
		}
	}
	return nil
}

// Remove removes a learning by ID.
func (s *LearningsStore) Remove(id string) bool {
	for i, l := range s.Learnings {
		if l.ID == id {
			s.Learnings = append(s.Learnings[:i], s.Learnings[i+1:]...)
			s.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// SortByRelevance sorts learnings by a score function.
func (s *LearningsStore) SortByRelevance(scoreFn func(Learning) int) {
	type scored struct {
		learning Learning
		score    int
	}

	var results []scored
	for _, l := range s.Learnings {
		results = append(results, scored{l, scoreFn(l)})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	s.Learnings = make([]Learning, len(results))
	for i, r := range results {
		s.Learnings[i] = r.learning
	}
}

// LearningsPath returns the path to the learnings file for a workspace.
func LearningsPath(workspaceDir string) string {
	return filepath.Join(workspaceDir, ".forge", "context", DefaultLearningsFile)
}

// generateLearningID generates a unique learning ID.
func generateLearningID() string {
	return "learning-" + time.Now().Format("20060102150405")
}
