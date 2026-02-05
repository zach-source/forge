package worker

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/zach-source/forge/internal/ralph"
	"gopkg.in/yaml.v3"
)

// TaskResult represents the outcome of a task.
type TaskResult string

const (
	TaskResultSuccess TaskResult = "success"
	TaskResultFailure TaskResult = "failure"
)

// TaskMetric represents metrics for a single completed task.
type TaskMetric struct {
	TaskID     string        `yaml:"task_id"`
	WorkerID   string        `yaml:"worker_id"`
	StartedAt  time.Time     `yaml:"started_at"`
	FinishedAt time.Time     `yaml:"finished_at"`
	Duration   time.Duration `yaml:"duration"`
	Iterations int           `yaml:"iterations"`
	Result     TaskResult    `yaml:"result"`
	Error      string        `yaml:"error,omitempty"`
}

// WorkerMetrics represents aggregate metrics for a single worker.
type WorkerMetrics struct {
	WorkerID       string        `yaml:"worker_id"`
	WorkerName     string        `yaml:"worker_name"`
	TasksCompleted int           `yaml:"tasks_completed"`
	TasksSucceeded int           `yaml:"tasks_succeeded"`
	TasksFailed    int           `yaml:"tasks_failed"`
	TotalDuration  time.Duration `yaml:"total_duration"`
	TotalIters     int           `yaml:"total_iterations"`
	LastError      string        `yaml:"last_error,omitempty"`
	LastTaskAt     time.Time     `yaml:"last_task_at,omitempty"`
}

// SuccessRate returns the success rate as a percentage (0-100).
func (m *WorkerMetrics) SuccessRate() float64 {
	if m.TasksCompleted == 0 {
		return 0
	}
	return float64(m.TasksSucceeded) / float64(m.TasksCompleted) * 100
}

// AvgDuration returns the average task duration.
func (m *WorkerMetrics) AvgDuration() time.Duration {
	if m.TasksCompleted == 0 {
		return 0
	}
	return m.TotalDuration / time.Duration(m.TasksCompleted)
}

// AvgIterations returns the average iterations per task.
func (m *WorkerMetrics) AvgIterations() float64 {
	if m.TasksCompleted == 0 {
		return 0
	}
	return float64(m.TotalIters) / float64(m.TasksCompleted)
}

// AggregateMetrics represents overall metrics across all workers.
type AggregateMetrics struct {
	TotalTasksToday    int           `yaml:"total_tasks_today"`
	TotalTasksAllTime  int           `yaml:"total_tasks_all_time"`
	OverallSuccessRate float64       `yaml:"overall_success_rate"`
	AvgThroughput      float64       `yaml:"avg_throughput"`     // tasks per hour
	WorkerUtilization  float64       `yaml:"worker_utilization"` // percentage
	TotalActiveTime    time.Duration `yaml:"total_active_time"`
}

// MetricsData is the YAML structure for the metrics file.
type MetricsData struct {
	Workers     map[string]*WorkerMetrics `yaml:"workers"`
	TaskHistory []TaskMetric              `yaml:"task_history"`
	UpdatedAt   time.Time                 `yaml:"updated_at"`
}

// MetricsStore manages worker performance metrics.
type MetricsStore struct {
	data *MetricsData
	path string
	mu   sync.RWMutex
}

// DefaultMetricsPath returns the default path for metrics storage.
func DefaultMetricsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".forge", "workers", "metrics.yaml")
}

// NewMetricsStore creates a new metrics store at the default path.
func NewMetricsStore() (*MetricsStore, error) {
	return NewMetricsStoreFrom(DefaultMetricsPath())
}

// NewMetricsStoreFrom creates a metrics store at the specified path.
func NewMetricsStoreFrom(path string) (*MetricsStore, error) {
	store := &MetricsStore{
		path: path,
		data: &MetricsData{
			Workers:     make(map[string]*WorkerMetrics),
			TaskHistory: []TaskMetric{},
		},
	}

	// Load existing data if present
	if err := store.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("loading metrics: %w", err)
	}

	return store, nil
}

// load reads metrics from disk.
func (s *MetricsStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}

	var metricsData MetricsData
	if err := yaml.Unmarshal(data, &metricsData); err != nil {
		return fmt.Errorf("parsing metrics: %w", err)
	}

	if metricsData.Workers == nil {
		metricsData.Workers = make(map[string]*WorkerMetrics)
	}

	s.data = &metricsData
	return nil
}

// Save persists metrics to disk.
func (s *MetricsStore) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.saveUnlocked()
}

func (s *MetricsStore) saveUnlocked() error {
	// Ensure directory exists
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating metrics directory: %w", err)
	}

	s.data.UpdatedAt = time.Now()

	content, err := yaml.Marshal(s.data)
	if err != nil {
		return fmt.Errorf("marshaling metrics: %w", err)
	}

	// Write atomically
	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, content, 0o644); err != nil {
		return fmt.Errorf("writing metrics: %w", err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming metrics: %w", err)
	}

	return nil
}

// RecordTaskStart records when a worker starts a task.
// Returns a TaskMetric that should be passed to RecordTaskComplete.
func (s *MetricsStore) RecordTaskStart(workerID, workerName, taskID string) *TaskMetric {
	return &TaskMetric{
		TaskID:    taskID,
		WorkerID:  workerID,
		StartedAt: time.Now(),
	}
}

// RecordTaskComplete records the completion of a task.
func (s *MetricsStore) RecordTaskComplete(metric *TaskMetric, result TaskResult, iterations int, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	metric.FinishedAt = time.Now()
	metric.Duration = metric.FinishedAt.Sub(metric.StartedAt)
	metric.Iterations = iterations
	metric.Result = result
	metric.Error = errMsg

	// Update worker metrics
	wm, ok := s.data.Workers[metric.WorkerID]
	if !ok {
		wm = &WorkerMetrics{
			WorkerID: metric.WorkerID,
		}
		s.data.Workers[metric.WorkerID] = wm
	}

	wm.TasksCompleted++
	wm.TotalDuration += metric.Duration
	wm.TotalIters += iterations
	wm.LastTaskAt = metric.FinishedAt

	if result == TaskResultSuccess {
		wm.TasksSucceeded++
	} else {
		wm.TasksFailed++
		wm.LastError = errMsg
	}

	// Add to history (keep last 100 tasks)
	s.data.TaskHistory = append(s.data.TaskHistory, *metric)
	if len(s.data.TaskHistory) > 100 {
		s.data.TaskHistory = s.data.TaskHistory[len(s.data.TaskHistory)-100:]
	}

	return s.saveUnlocked()
}

// RecordTaskCompleteSimple records task completion with just the essential info.
// This is a convenience method when you don't have the start metric.
func (s *MetricsStore) RecordTaskCompleteSimple(workerID, workerName, taskID string, startedAt time.Time, result TaskResult, iterations int, errMsg string) error {
	metric := &TaskMetric{
		TaskID:    taskID,
		WorkerID:  workerID,
		StartedAt: startedAt,
	}
	return s.RecordTaskComplete(metric, result, iterations, errMsg)
}

// GetWorkerMetrics returns metrics for a specific worker.
func (s *MetricsStore) GetWorkerMetrics(workerID string) *WorkerMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if wm, ok := s.data.Workers[workerID]; ok {
		// Return a copy
		copy := *wm
		return &copy
	}
	return nil
}

// GetAllWorkerMetrics returns metrics for all workers.
func (s *MetricsStore) GetAllWorkerMetrics() map[string]*WorkerMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*WorkerMetrics, len(s.data.Workers))
	for id, wm := range s.data.Workers {
		copy := *wm
		result[id] = &copy
	}
	return result
}

// GetAggregateMetrics calculates aggregate metrics across all workers.
func (s *MetricsStore) GetAggregateMetrics() *AggregateMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agg := &AggregateMetrics{}

	totalSucceeded := 0
	totalCompleted := 0

	for _, wm := range s.data.Workers {
		totalCompleted += wm.TasksCompleted
		totalSucceeded += wm.TasksSucceeded
		agg.TotalActiveTime += wm.TotalDuration
	}

	agg.TotalTasksAllTime = totalCompleted

	if totalCompleted > 0 {
		agg.OverallSuccessRate = float64(totalSucceeded) / float64(totalCompleted) * 100
	}

	// Count tasks completed today
	today := time.Now().Truncate(24 * time.Hour)
	for _, task := range s.data.TaskHistory {
		if task.FinishedAt.After(today) {
			agg.TotalTasksToday++
		}
	}

	// Calculate throughput (tasks per hour) based on today's tasks
	hoursElapsed := time.Since(today).Hours()
	if hoursElapsed > 0 && agg.TotalTasksToday > 0 {
		agg.AvgThroughput = float64(agg.TotalTasksToday) / hoursElapsed
	}

	return agg
}

// GetTasksCompletedToday returns the count of tasks completed today.
func (s *MetricsStore) GetTasksCompletedToday() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	today := time.Now().Truncate(24 * time.Hour)
	count := 0
	for _, task := range s.data.TaskHistory {
		if task.FinishedAt.After(today) {
			count++
		}
	}
	return count
}

// SetWorkerName updates the worker name in metrics (for display purposes).
func (s *MetricsStore) SetWorkerName(workerID, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if wm, ok := s.data.Workers[workerID]; ok {
		wm.WorkerName = name
	}
}

// Path returns the metrics file path.
func (s *MetricsStore) Path() string {
	return s.path
}

// GetSessionIterations returns the iteration count from a worker's session state file.
// Returns 0 if the state file cannot be read.
func GetSessionIterations(sessionID string) int {
	statePath := ralph.SessionStatePath(sessionID)
	ctrl := ralph.NewStateController(statePath)
	state, err := ctrl.Read()
	if err != nil {
		return 0
	}
	return state.Iteration
}

// RecordWorkerTaskComplete is a convenience function for recording task completion.
// It handles loading/creating the metrics store, recording the completion, and saving.
func RecordWorkerTaskComplete(workerID, workerName, taskID string, startedAt time.Time, sessionID string, success bool) error {
	store, err := NewMetricsStore()
	if err != nil {
		return fmt.Errorf("loading metrics store: %w", err)
	}

	// Try to get iteration count from session state
	iterations := GetSessionIterations(sessionID)

	result := TaskResultSuccess
	errMsg := ""
	if !success {
		result = TaskResultFailure
		errMsg = "task failed or was cancelled"
	}

	// Update worker name in metrics
	store.SetWorkerName(workerID, workerName)

	return store.RecordTaskCompleteSimple(workerID, workerName, taskID, startedAt, result, iterations, errMsg)
}
