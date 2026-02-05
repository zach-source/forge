package worker

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewMetricsStore(t *testing.T) {
	// Use temp directory for test
	tmpDir := t.TempDir()
	metricsPath := filepath.Join(tmpDir, "metrics.yaml")

	store, err := NewMetricsStoreFrom(metricsPath)
	if err != nil {
		t.Fatalf("NewMetricsStoreFrom() error = %v", err)
	}

	if store.Path() != metricsPath {
		t.Errorf("Path() = %v, want %v", store.Path(), metricsPath)
	}

	// Should have empty data
	metrics := store.GetAllWorkerMetrics()
	if len(metrics) != 0 {
		t.Errorf("GetAllWorkerMetrics() = %v, want empty", metrics)
	}
}

func TestMetricsStore_RecordTaskComplete(t *testing.T) {
	tmpDir := t.TempDir()
	metricsPath := filepath.Join(tmpDir, "metrics.yaml")

	store, err := NewMetricsStoreFrom(metricsPath)
	if err != nil {
		t.Fatalf("NewMetricsStoreFrom() error = %v", err)
	}

	// Start and complete a task
	workerID := "w-test123"
	workerName := "alpha"
	taskID := "task-001"
	startTime := time.Now().Add(-10 * time.Minute)

	metric := store.RecordTaskStart(workerID, workerName, taskID)
	if metric.TaskID != taskID {
		t.Errorf("TaskID = %v, want %v", metric.TaskID, taskID)
	}
	if metric.WorkerID != workerID {
		t.Errorf("WorkerID = %v, want %v", metric.WorkerID, workerID)
	}

	// Override start time for consistent test
	metric.StartedAt = startTime

	err = store.RecordTaskComplete(metric, TaskResultSuccess, 50, "")
	if err != nil {
		t.Fatalf("RecordTaskComplete() error = %v", err)
	}

	// Verify worker metrics
	wm := store.GetWorkerMetrics(workerID)
	if wm == nil {
		t.Fatalf("GetWorkerMetrics() returned nil")
	}

	if wm.TasksCompleted != 1 {
		t.Errorf("TasksCompleted = %v, want 1", wm.TasksCompleted)
	}
	if wm.TasksSucceeded != 1 {
		t.Errorf("TasksSucceeded = %v, want 1", wm.TasksSucceeded)
	}
	if wm.TasksFailed != 0 {
		t.Errorf("TasksFailed = %v, want 0", wm.TasksFailed)
	}
	if wm.TotalIters != 50 {
		t.Errorf("TotalIters = %v, want 50", wm.TotalIters)
	}
}

func TestMetricsStore_RecordTaskFailure(t *testing.T) {
	tmpDir := t.TempDir()
	metricsPath := filepath.Join(tmpDir, "metrics.yaml")

	store, err := NewMetricsStoreFrom(metricsPath)
	if err != nil {
		t.Fatalf("NewMetricsStoreFrom() error = %v", err)
	}

	workerID := "w-test123"
	workerName := "alpha"
	taskID := "task-001"
	errMsg := "task timed out"

	metric := store.RecordTaskStart(workerID, workerName, taskID)
	err = store.RecordTaskComplete(metric, TaskResultFailure, 10, errMsg)
	if err != nil {
		t.Fatalf("RecordTaskComplete() error = %v", err)
	}

	wm := store.GetWorkerMetrics(workerID)
	if wm.TasksFailed != 1 {
		t.Errorf("TasksFailed = %v, want 1", wm.TasksFailed)
	}
	if wm.LastError != errMsg {
		t.Errorf("LastError = %v, want %v", wm.LastError, errMsg)
	}
}

func TestWorkerMetrics_SuccessRate(t *testing.T) {
	tests := []struct {
		name      string
		completed int
		succeeded int
		want      float64
	}{
		{"no tasks", 0, 0, 0},
		{"all success", 10, 10, 100},
		{"half success", 10, 5, 50},
		{"one third", 3, 1, 33.33},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &WorkerMetrics{
				TasksCompleted: tt.completed,
				TasksSucceeded: tt.succeeded,
			}
			got := m.SuccessRate()
			// Use delta comparison for floating point
			delta := got - tt.want
			if delta < 0 {
				delta = -delta
			}
			if delta > 0.01 {
				t.Errorf("SuccessRate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWorkerMetrics_AvgDuration(t *testing.T) {
	tests := []struct {
		name      string
		completed int
		duration  time.Duration
		want      time.Duration
	}{
		{"no tasks", 0, 0, 0},
		{"single task", 1, 10 * time.Minute, 10 * time.Minute},
		{"multiple tasks", 5, 50 * time.Minute, 10 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &WorkerMetrics{
				TasksCompleted: tt.completed,
				TotalDuration:  tt.duration,
			}
			got := m.AvgDuration()
			if got != tt.want {
				t.Errorf("AvgDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWorkerMetrics_AvgIterations(t *testing.T) {
	tests := []struct {
		name      string
		completed int
		iters     int
		want      float64
	}{
		{"no tasks", 0, 0, 0},
		{"single task", 1, 50, 50},
		{"multiple tasks", 5, 250, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &WorkerMetrics{
				TasksCompleted: tt.completed,
				TotalIters:     tt.iters,
			}
			got := m.AvgIterations()
			if got != tt.want {
				t.Errorf("AvgIterations() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMetricsStore_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	metricsPath := filepath.Join(tmpDir, "metrics.yaml")

	// Create store and record a task
	store1, err := NewMetricsStoreFrom(metricsPath)
	if err != nil {
		t.Fatalf("NewMetricsStoreFrom() error = %v", err)
	}

	workerID := "w-test123"
	metric := store1.RecordTaskStart(workerID, "alpha", "task-001")
	if err := store1.RecordTaskComplete(metric, TaskResultSuccess, 42, ""); err != nil {
		t.Fatalf("RecordTaskComplete() error = %v", err)
	}

	// Create new store from same path (simulates restart)
	store2, err := NewMetricsStoreFrom(metricsPath)
	if err != nil {
		t.Fatalf("NewMetricsStoreFrom() error = %v", err)
	}

	// Verify data was persisted
	wm := store2.GetWorkerMetrics(workerID)
	if wm == nil {
		t.Fatalf("GetWorkerMetrics() returned nil after reload")
	}

	if wm.TasksCompleted != 1 {
		t.Errorf("TasksCompleted after reload = %v, want 1", wm.TasksCompleted)
	}
	if wm.TotalIters != 42 {
		t.Errorf("TotalIters after reload = %v, want 42", wm.TotalIters)
	}
}

func TestMetricsStore_GetAggregateMetrics(t *testing.T) {
	tmpDir := t.TempDir()
	metricsPath := filepath.Join(tmpDir, "metrics.yaml")

	store, err := NewMetricsStoreFrom(metricsPath)
	if err != nil {
		t.Fatalf("NewMetricsStoreFrom() error = %v", err)
	}

	// Record tasks for multiple workers
	for i := 0; i < 3; i++ {
		workerID := "w-test" + string(rune('0'+i))
		metric := store.RecordTaskStart(workerID, "", "task")
		store.RecordTaskComplete(metric, TaskResultSuccess, 10, "")
	}

	// Record one failure
	metric := store.RecordTaskStart("w-test0", "", "task-fail")
	store.RecordTaskComplete(metric, TaskResultFailure, 5, "error")

	agg := store.GetAggregateMetrics()

	if agg.TotalTasksAllTime != 4 {
		t.Errorf("TotalTasksAllTime = %v, want 4", agg.TotalTasksAllTime)
	}

	expectedRate := 75.0 // 3 success, 1 failure
	if agg.OverallSuccessRate != expectedRate {
		t.Errorf("OverallSuccessRate = %v, want %v", agg.OverallSuccessRate, expectedRate)
	}
}

func TestMetricsStore_TaskHistoryLimit(t *testing.T) {
	tmpDir := t.TempDir()
	metricsPath := filepath.Join(tmpDir, "metrics.yaml")

	store, err := NewMetricsStoreFrom(metricsPath)
	if err != nil {
		t.Fatalf("NewMetricsStoreFrom() error = %v", err)
	}

	// Record more than 100 tasks
	for i := 0; i < 150; i++ {
		metric := store.RecordTaskStart("w-test", "", "task")
		store.RecordTaskComplete(metric, TaskResultSuccess, 1, "")
	}

	// Reload and check history is limited
	store2, err := NewMetricsStoreFrom(metricsPath)
	if err != nil {
		t.Fatalf("NewMetricsStoreFrom() error = %v", err)
	}

	// Access internal data to check history length
	if len(store2.data.TaskHistory) > 100 {
		t.Errorf("TaskHistory length = %v, want <= 100", len(store2.data.TaskHistory))
	}
}

func TestMetricsStore_FileCreation(t *testing.T) {
	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "subdir", "metrics.yaml")

	store, err := NewMetricsStoreFrom(nestedPath)
	if err != nil {
		t.Fatalf("NewMetricsStoreFrom() error = %v", err)
	}

	// Save should create the directory
	metric := store.RecordTaskStart("w-test", "", "task")
	if err := store.RecordTaskComplete(metric, TaskResultSuccess, 1, ""); err != nil {
		t.Fatalf("RecordTaskComplete() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(nestedPath); os.IsNotExist(err) {
		t.Errorf("Metrics file not created at %v", nestedPath)
	}
}
