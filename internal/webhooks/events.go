package webhooks

import (
	"time"
)

// Event represents a webhook event with its metadata.
type Event struct {
	Type       EventType
	Priority   Priority
	WorkerName string
	LeaderRole string
	Data       interface{}
}

// Payload is the JSON structure sent to webhook endpoints.
type Payload struct {
	Event     EventType   `json:"event"`
	Timestamp string      `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// ToPayload converts an Event to a webhook Payload.
func (e Event) ToPayload() Payload {
	return Payload{
		Event:     e.Type,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      e.Data,
	}
}

// TaskCompletedData contains data for task_completed events.
type TaskCompletedData struct {
	TaskID          string `json:"task_id"`
	Title           string `json:"title"`
	Worker          string `json:"worker"`
	DurationMinutes int    `json:"duration_minutes,omitempty"`
	Priority        string `json:"priority,omitempty"`
}

// TaskFailedData contains data for task_failed events.
type TaskFailedData struct {
	TaskID   string `json:"task_id"`
	Title    string `json:"title"`
	Reason   string `json:"reason"`
	Worker   string `json:"worker,omitempty"`
	Priority string `json:"priority,omitempty"`
}

// WorkerStartedData contains data for worker_started events.
type WorkerStartedData struct {
	WorkerID   string `json:"worker_id"`
	WorkerName string `json:"worker_name"`
	TaskID     string `json:"task_id,omitempty"`
	TaskTitle  string `json:"task_title,omitempty"`
	Worktree   string `json:"worktree,omitempty"`
}

// WorkerStoppedData contains data for worker_stopped events.
type WorkerStoppedData struct {
	WorkerID   string `json:"worker_id"`
	WorkerName string `json:"worker_name"`
	TaskID     string `json:"task_id,omitempty"`
	Reason     string `json:"reason"`
}

// WorkerErrorData contains data for worker_error events.
type WorkerErrorData struct {
	WorkerID   string `json:"worker_id"`
	WorkerName string `json:"worker_name"`
	TaskID     string `json:"task_id,omitempty"`
	Error      string `json:"error"`
}

// LeaderLaunchedData contains data for leader_launched events.
type LeaderLaunchedData struct {
	Role       string `json:"role"`
	WorkerID   string `json:"worker_id"`
	WorkerName string `json:"worker_name"`
	SessionID  string `json:"session_id"`
}

// DeploymentTriggeredData contains data for deployment_triggered events.
type DeploymentTriggeredData struct {
	WorkerID   string   `json:"worker_id"`
	WorkerName string   `json:"worker_name"`
	SessionID  string   `json:"session_id"`
	TaskCount  int      `json:"task_count,omitempty"`
	Tasks      []string `json:"tasks,omitempty"`
}

// BuildFailedData contains data for build_failed events.
type BuildFailedData struct {
	TaskID    string `json:"task_id,omitempty"`
	Branch    string `json:"branch,omitempty"`
	Error     string `json:"error"`
	Worker    string `json:"worker,omitempty"`
	BuildURL  string `json:"build_url,omitempty"`
	CommitSHA string `json:"commit_sha,omitempty"`
}

// NewTaskCompletedEvent creates a task_completed event.
func NewTaskCompletedEvent(taskID, title, workerName string, priority Priority, durationMinutes int) Event {
	return Event{
		Type:       EventTaskCompleted,
		Priority:   priority,
		WorkerName: workerName,
		Data: TaskCompletedData{
			TaskID:          taskID,
			Title:           title,
			Worker:          workerName,
			DurationMinutes: durationMinutes,
			Priority:        string(priority),
		},
	}
}

// NewTaskFailedEvent creates a task_failed event.
func NewTaskFailedEvent(taskID, title, reason, workerName string, priority Priority) Event {
	return Event{
		Type:       EventTaskFailed,
		Priority:   priority,
		WorkerName: workerName,
		Data: TaskFailedData{
			TaskID:   taskID,
			Title:    title,
			Reason:   reason,
			Worker:   workerName,
			Priority: string(priority),
		},
	}
}

// NewWorkerStartedEvent creates a worker_started event.
func NewWorkerStartedEvent(workerID, workerName, taskID, taskTitle, worktree string) Event {
	return Event{
		Type:       EventWorkerStarted,
		WorkerName: workerName,
		Data: WorkerStartedData{
			WorkerID:   workerID,
			WorkerName: workerName,
			TaskID:     taskID,
			TaskTitle:  taskTitle,
			Worktree:   worktree,
		},
	}
}

// NewWorkerStoppedEvent creates a worker_stopped event.
func NewWorkerStoppedEvent(workerID, workerName, taskID, reason string) Event {
	return Event{
		Type:       EventWorkerStopped,
		WorkerName: workerName,
		Data: WorkerStoppedData{
			WorkerID:   workerID,
			WorkerName: workerName,
			TaskID:     taskID,
			Reason:     reason,
		},
	}
}

// NewWorkerErrorEvent creates a worker_error event.
func NewWorkerErrorEvent(workerID, workerName, taskID, errMsg string) Event {
	return Event{
		Type:       EventWorkerError,
		WorkerName: workerName,
		Data: WorkerErrorData{
			WorkerID:   workerID,
			WorkerName: workerName,
			TaskID:     taskID,
			Error:      errMsg,
		},
	}
}

// NewLeaderLaunchedEvent creates a leader_launched event.
func NewLeaderLaunchedEvent(role, workerID, workerName, sessionID string) Event {
	return Event{
		Type:       EventLeaderLaunched,
		LeaderRole: role,
		Data: LeaderLaunchedData{
			Role:       role,
			WorkerID:   workerID,
			WorkerName: workerName,
			SessionID:  sessionID,
		},
	}
}

// NewDeploymentTriggeredEvent creates a deployment_triggered event.
func NewDeploymentTriggeredEvent(workerID, workerName, sessionID string, taskCount int, tasks []string) Event {
	return Event{
		Type:       EventDeploymentTriggered,
		LeaderRole: "deploy",
		Data: DeploymentTriggeredData{
			WorkerID:   workerID,
			WorkerName: workerName,
			SessionID:  sessionID,
			TaskCount:  taskCount,
			Tasks:      tasks,
		},
	}
}

// NewBuildFailedEvent creates a build_failed event.
func NewBuildFailedEvent(taskID, branch, errMsg, workerName, buildURL, commitSHA string) Event {
	return Event{
		Type:       EventBuildFailed,
		WorkerName: workerName,
		Data: BuildFailedData{
			TaskID:    taskID,
			Branch:    branch,
			Error:     errMsg,
			Worker:    workerName,
			BuildURL:  buildURL,
			CommitSHA: commitSHA,
		},
	}
}
