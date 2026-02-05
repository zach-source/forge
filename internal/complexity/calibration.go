package complexity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Record tracks estimated vs actual complexity for calibration.
type Record struct {
	TaskID           string     `json:"task_id"`
	Estimated        Complexity `json:"estimated"`
	Actual           Complexity `json:"actual,omitempty"`
	EstimatedMinutes int        `json:"estimated_minutes"`
	ActualMinutes    int        `json:"actual_minutes,omitempty"`
	StartedAt        time.Time  `json:"started_at,omitempty"`
	CompletedAt      time.Time  `json:"completed_at,omitempty"`
}

// Tracker stores calibration records to a JSON file.
type Tracker struct {
	path    string
	records []Record
}

// NewTracker loads or creates a calibration tracker at the given directory.
// Records are stored in <dir>/.forge/complexity/calibration.json.
func NewTracker(workDir string) (*Tracker, error) {
	path := filepath.Join(workDir, ".forge", "complexity", "calibration.json")

	t := &Tracker{path: path}
	if err := t.load(); err != nil {
		return nil, err
	}
	return t, nil
}

// RecordEstimate records a new complexity estimate for a task.
func (t *Tracker) RecordEstimate(taskID string, estimated Complexity) {
	// Update existing record or create new
	for i, r := range t.records {
		if r.TaskID == taskID {
			t.records[i].Estimated = estimated
			t.records[i].EstimatedMinutes = estimatedMinutes(estimated)
			return
		}
	}

	t.records = append(t.records, Record{
		TaskID:           taskID,
		Estimated:        estimated,
		EstimatedMinutes: estimatedMinutes(estimated),
	})
}

// RecordStart records when a task starts.
func (t *Tracker) RecordStart(taskID string) {
	for i, r := range t.records {
		if r.TaskID == taskID {
			t.records[i].StartedAt = time.Now()
			return
		}
	}
}

// RecordCompletion records when a task completes with actual complexity.
func (t *Tracker) RecordCompletion(taskID string, actual Complexity) {
	for i, r := range t.records {
		if r.TaskID == taskID {
			t.records[i].Actual = actual
			t.records[i].CompletedAt = time.Now()
			if !r.StartedAt.IsZero() {
				t.records[i].ActualMinutes = int(time.Since(r.StartedAt).Minutes())
			}
			return
		}
	}
}

// Save writes records to disk.
func (t *Tracker) Save() error {
	dir := filepath.Dir(t.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating calibration directory: %w", err)
	}

	data, err := json.MarshalIndent(t.records, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling calibration data: %w", err)
	}

	return os.WriteFile(t.path, data, 0644)
}

// Records returns all calibration records.
func (t *Tracker) Records() []Record {
	return t.records
}

// Accuracy returns the percentage of tasks where estimated == actual.
// Returns -1 if no completed records exist.
func (t *Tracker) Accuracy() float64 {
	completed := 0
	correct := 0
	for _, r := range t.records {
		if r.Actual != "" {
			completed++
			if r.Estimated == r.Actual {
				correct++
			}
		}
	}
	if completed == 0 {
		return -1
	}
	return float64(correct) / float64(completed)
}

func (t *Tracker) load() error {
	data, err := os.ReadFile(t.path)
	if err != nil {
		if os.IsNotExist(err) {
			t.records = nil
			return nil
		}
		return fmt.Errorf("reading calibration data: %w", err)
	}

	return json.Unmarshal(data, &t.records)
}

// estimatedMinutes returns a rough minute estimate for a complexity level.
func estimatedMinutes(c Complexity) int {
	switch c {
	case ComplexitySmall:
		return 30
	case ComplexityMedium:
		return 60
	case ComplexityLarge:
		return 120
	case ComplexityXL:
		return 240
	default:
		return 60
	}
}
