package webhooks

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEventToPayload(t *testing.T) {
	event := Event{
		Type:       EventTaskCompleted,
		Priority:   PriorityHigh,
		WorkerName: "alpha",
		Data: TaskCompletedData{
			TaskID:          "task-123",
			Title:           "Fix bug",
			Worker:          "alpha",
			DurationMinutes: 15,
			Priority:        "high",
		},
	}

	payload := event.ToPayload()

	if payload.Event != EventTaskCompleted {
		t.Errorf("expected event type %s, got %s", EventTaskCompleted, payload.Event)
	}

	// Verify timestamp is valid RFC3339
	_, err := time.Parse(time.RFC3339, payload.Timestamp)
	if err != nil {
		t.Errorf("timestamp is not valid RFC3339: %s", payload.Timestamp)
	}

	// Verify data is preserved
	data, ok := payload.Data.(TaskCompletedData)
	if !ok {
		t.Fatal("payload data is not TaskCompletedData")
	}
	if data.TaskID != "task-123" {
		t.Errorf("expected task_id 'task-123', got '%s'", data.TaskID)
	}
}

func TestPayloadJSON(t *testing.T) {
	event := NewTaskCompletedEvent("task-123", "Fix bug", "alpha", PriorityHigh, 15)
	payload := event.ToPayload()

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if decoded["event"] != string(EventTaskCompleted) {
		t.Errorf("expected event 'task_completed', got '%v'", decoded["event"])
	}

	data, ok := decoded["data"].(map[string]interface{})
	if !ok {
		t.Fatal("data is not a map")
	}
	if data["task_id"] != "task-123" {
		t.Errorf("expected task_id 'task-123', got '%v'", data["task_id"])
	}
}

func TestNewTaskCompletedEvent(t *testing.T) {
	event := NewTaskCompletedEvent("task-123", "Fix bug", "alpha", PriorityHigh, 15)

	if event.Type != EventTaskCompleted {
		t.Errorf("expected event type %s, got %s", EventTaskCompleted, event.Type)
	}
	if event.Priority != PriorityHigh {
		t.Errorf("expected priority %s, got %s", PriorityHigh, event.Priority)
	}
	if event.WorkerName != "alpha" {
		t.Errorf("expected worker 'alpha', got '%s'", event.WorkerName)
	}

	data, ok := event.Data.(TaskCompletedData)
	if !ok {
		t.Fatal("data is not TaskCompletedData")
	}
	if data.TaskID != "task-123" {
		t.Errorf("expected task_id 'task-123', got '%s'", data.TaskID)
	}
	if data.DurationMinutes != 15 {
		t.Errorf("expected duration 15, got %d", data.DurationMinutes)
	}
}

func TestNewTaskFailedEvent(t *testing.T) {
	event := NewTaskFailedEvent("task-123", "Fix bug", "stuck without worker", "alpha", PriorityHigh)

	if event.Type != EventTaskFailed {
		t.Errorf("expected event type %s, got %s", EventTaskFailed, event.Type)
	}

	data, ok := event.Data.(TaskFailedData)
	if !ok {
		t.Fatal("data is not TaskFailedData")
	}
	if data.Reason != "stuck without worker" {
		t.Errorf("expected reason 'stuck without worker', got '%s'", data.Reason)
	}
}

func TestNewWorkerStartedEvent(t *testing.T) {
	event := NewWorkerStartedEvent("w-123", "alpha", "task-456", "Fix bug", "/path/to/worktree")

	if event.Type != EventWorkerStarted {
		t.Errorf("expected event type %s, got %s", EventWorkerStarted, event.Type)
	}

	data, ok := event.Data.(WorkerStartedData)
	if !ok {
		t.Fatal("data is not WorkerStartedData")
	}
	if data.WorkerID != "w-123" {
		t.Errorf("expected worker_id 'w-123', got '%s'", data.WorkerID)
	}
	if data.Worktree != "/path/to/worktree" {
		t.Errorf("expected worktree '/path/to/worktree', got '%s'", data.Worktree)
	}
}

func TestNewWorkerStoppedEvent(t *testing.T) {
	event := NewWorkerStoppedEvent("w-123", "alpha", "task-456", "Claude exited")

	if event.Type != EventWorkerStopped {
		t.Errorf("expected event type %s, got %s", EventWorkerStopped, event.Type)
	}

	data, ok := event.Data.(WorkerStoppedData)
	if !ok {
		t.Fatal("data is not WorkerStoppedData")
	}
	if data.Reason != "Claude exited" {
		t.Errorf("expected reason 'Claude exited', got '%s'", data.Reason)
	}
}

func TestNewWorkerErrorEvent(t *testing.T) {
	event := NewWorkerErrorEvent("w-123", "alpha", "task-456", "connection timeout")

	if event.Type != EventWorkerError {
		t.Errorf("expected event type %s, got %s", EventWorkerError, event.Type)
	}

	data, ok := event.Data.(WorkerErrorData)
	if !ok {
		t.Fatal("data is not WorkerErrorData")
	}
	if data.Error != "connection timeout" {
		t.Errorf("expected error 'connection timeout', got '%s'", data.Error)
	}
}

func TestNewLeaderLaunchedEvent(t *testing.T) {
	event := NewLeaderLaunchedEvent("reviewer", "w-123", "alpha", "forge-reviewer-alpha")

	if event.Type != EventLeaderLaunched {
		t.Errorf("expected event type %s, got %s", EventLeaderLaunched, event.Type)
	}
	if event.LeaderRole != "reviewer" {
		t.Errorf("expected leader role 'reviewer', got '%s'", event.LeaderRole)
	}

	data, ok := event.Data.(LeaderLaunchedData)
	if !ok {
		t.Fatal("data is not LeaderLaunchedData")
	}
	if data.Role != "reviewer" {
		t.Errorf("expected role 'reviewer', got '%s'", data.Role)
	}
	if data.SessionID != "forge-reviewer-alpha" {
		t.Errorf("expected session_id 'forge-reviewer-alpha', got '%s'", data.SessionID)
	}
}

func TestNewDeploymentTriggeredEvent(t *testing.T) {
	tasks := []string{"task-1", "task-2", "task-3"}
	event := NewDeploymentTriggeredEvent("w-123", "delta", "forge-deploy-delta", 3, tasks)

	if event.Type != EventDeploymentTriggered {
		t.Errorf("expected event type %s, got %s", EventDeploymentTriggered, event.Type)
	}
	if event.LeaderRole != "deploy" {
		t.Errorf("expected leader role 'deploy', got '%s'", event.LeaderRole)
	}

	data, ok := event.Data.(DeploymentTriggeredData)
	if !ok {
		t.Fatal("data is not DeploymentTriggeredData")
	}
	if data.TaskCount != 3 {
		t.Errorf("expected task_count 3, got %d", data.TaskCount)
	}
	if len(data.Tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(data.Tasks))
	}
}

func TestNewBuildFailedEvent(t *testing.T) {
	event := NewBuildFailedEvent("task-123", "feature/xyz", "test failure", "alpha", "https://ci.example.com/123", "abc123")

	if event.Type != EventBuildFailed {
		t.Errorf("expected event type %s, got %s", EventBuildFailed, event.Type)
	}

	data, ok := event.Data.(BuildFailedData)
	if !ok {
		t.Fatal("data is not BuildFailedData")
	}
	if data.Branch != "feature/xyz" {
		t.Errorf("expected branch 'feature/xyz', got '%s'", data.Branch)
	}
	if data.BuildURL != "https://ci.example.com/123" {
		t.Errorf("expected build_url 'https://ci.example.com/123', got '%s'", data.BuildURL)
	}
	if data.CommitSHA != "abc123" {
		t.Errorf("expected commit_sha 'abc123', got '%s'", data.CommitSHA)
	}
}
