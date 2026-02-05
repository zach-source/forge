package webhooks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Test loading from non-existent directory returns nil, nil
	cfg, err := LoadConfig("/nonexistent/path")
	if err != nil {
		t.Errorf("LoadConfig should not error on missing file: %v", err)
	}
	if cfg != nil {
		t.Error("LoadConfig should return nil config for missing file")
	}

	// Create temp directory with config
	tmpDir := t.TempDir()
	foundryDir := filepath.Join(tmpDir, ".foundry")
	if err := os.MkdirAll(foundryDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write valid config
	configContent := `webhooks:
  - name: test-webhook
    url: https://example.com/hook
    events:
      - task_completed
      - worker_started
    filters:
      priority:
        - high
        - critical
    retry:
      max_attempts: 5
      backoff_seconds: 10
`
	if err := os.WriteFile(filepath.Join(foundryDir, "webhooks.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err = LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadConfig returned nil config")
	}

	if len(cfg.Webhooks) != 1 {
		t.Fatalf("expected 1 webhook, got %d", len(cfg.Webhooks))
	}

	wh := cfg.Webhooks[0]
	if wh.Name != "test-webhook" {
		t.Errorf("expected name 'test-webhook', got '%s'", wh.Name)
	}
	if wh.URL != "https://example.com/hook" {
		t.Errorf("expected URL 'https://example.com/hook', got '%s'", wh.URL)
	}
	if len(wh.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(wh.Events))
	}
	if wh.Retry.MaxAttempts != 5 {
		t.Errorf("expected max_attempts 5, got %d", wh.Retry.MaxAttempts)
	}
	if wh.Retry.BackoffSeconds != 10 {
		t.Errorf("expected backoff_seconds 10, got %d", wh.Retry.BackoffSeconds)
	}
}

func TestLoadConfigInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	foundryDir := filepath.Join(tmpDir, ".foundry")
	if err := os.MkdirAll(foundryDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write invalid YAML
	if err := os.WriteFile(filepath.Join(foundryDir, "webhooks.yaml"), []byte("invalid: yaml: content:"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(tmpDir)
	if err == nil {
		t.Error("LoadConfig should error on invalid YAML")
	}
}

func TestEndpointShouldNotify(t *testing.T) {
	tests := []struct {
		name       string
		endpoint   Endpoint
		event      EventType
		priority   Priority
		workerName string
		leaderRole string
		want       bool
	}{
		{
			name: "matching event",
			endpoint: Endpoint{
				Events: []EventType{EventTaskCompleted, EventWorkerStarted},
			},
			event: EventTaskCompleted,
			want:  true,
		},
		{
			name: "non-matching event",
			endpoint: Endpoint{
				Events: []EventType{EventTaskCompleted},
			},
			event: EventWorkerStarted,
			want:  false,
		},
		{
			name: "priority filter match",
			endpoint: Endpoint{
				Events: []EventType{EventTaskCompleted},
				Filters: FilterConfig{
					Priority: []Priority{PriorityHigh, PriorityCritical},
				},
			},
			event:    EventTaskCompleted,
			priority: PriorityHigh,
			want:     true,
		},
		{
			name: "priority filter no match",
			endpoint: Endpoint{
				Events: []EventType{EventTaskCompleted},
				Filters: FilterConfig{
					Priority: []Priority{PriorityHigh, PriorityCritical},
				},
			},
			event:    EventTaskCompleted,
			priority: PriorityLow,
			want:     false,
		},
		{
			name: "worker filter match",
			endpoint: Endpoint{
				Events: []EventType{EventWorkerStarted},
				Filters: FilterConfig{
					Workers: []string{"alpha", "bravo"},
				},
			},
			event:      EventWorkerStarted,
			workerName: "alpha",
			want:       true,
		},
		{
			name: "worker filter no match",
			endpoint: Endpoint{
				Events: []EventType{EventWorkerStarted},
				Filters: FilterConfig{
					Workers: []string{"alpha", "bravo"},
				},
			},
			event:      EventWorkerStarted,
			workerName: "charlie",
			want:       false,
		},
		{
			name: "leader filter match",
			endpoint: Endpoint{
				Events: []EventType{EventLeaderLaunched},
				Filters: FilterConfig{
					Leaders: []string{"reviewer", "planner"},
				},
			},
			event:      EventLeaderLaunched,
			leaderRole: "reviewer",
			want:       true,
		},
		{
			name: "leader filter no match",
			endpoint: Endpoint{
				Events: []EventType{EventLeaderLaunched},
				Filters: FilterConfig{
					Leaders: []string{"reviewer", "planner"},
				},
			},
			event:      EventLeaderLaunched,
			leaderRole: "deploy",
			want:       false,
		},
		{
			name: "no filters - all match",
			endpoint: Endpoint{
				Events: []EventType{EventTaskCompleted},
			},
			event:      EventTaskCompleted,
			priority:   PriorityLow,
			workerName: "any",
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.endpoint.ShouldNotify(tt.event, tt.priority, tt.workerName, tt.leaderRole)
			if got != tt.want {
				t.Errorf("ShouldNotify() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEndpointValidate(t *testing.T) {
	tests := []struct {
		name     string
		endpoint Endpoint
		wantErr  bool
	}{
		{
			name: "valid endpoint",
			endpoint: Endpoint{
				Name:   "test",
				URL:    "https://example.com",
				Events: []EventType{EventTaskCompleted},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			endpoint: Endpoint{
				URL:    "https://example.com",
				Events: []EventType{EventTaskCompleted},
			},
			wantErr: true,
		},
		{
			name: "missing URL",
			endpoint: Endpoint{
				Name:   "test",
				Events: []EventType{EventTaskCompleted},
			},
			wantErr: true,
		},
		{
			name: "no events",
			endpoint: Endpoint{
				Name: "test",
				URL:  "https://example.com",
			},
			wantErr: true,
		},
		{
			name: "invalid event type",
			endpoint: Endpoint{
				Name:   "test",
				URL:    "https://example.com",
				Events: []EventType{"invalid_event"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.endpoint.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Webhooks: []Endpoint{
					{Name: "test1", URL: "https://a.com", Events: []EventType{EventTaskCompleted}},
					{Name: "test2", URL: "https://b.com", Events: []EventType{EventWorkerStarted}},
				},
			},
			wantErr: false,
		},
		{
			name: "duplicate names",
			config: Config{
				Webhooks: []Endpoint{
					{Name: "test", URL: "https://a.com", Events: []EventType{EventTaskCompleted}},
					{Name: "test", URL: "https://b.com", Events: []EventType{EventWorkerStarted}},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid endpoint in config",
			config: Config{
				Webhooks: []Endpoint{
					{Name: "", URL: "https://a.com", Events: []EventType{EventTaskCompleted}},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAllEventTypes(t *testing.T) {
	events := AllEventTypes()
	if len(events) != 8 {
		t.Errorf("expected 8 event types, got %d", len(events))
	}

	// Verify all expected event types are present
	expected := map[EventType]bool{
		EventTaskCompleted:       true,
		EventTaskFailed:          true,
		EventWorkerStarted:       true,
		EventWorkerStopped:       true,
		EventWorkerError:         true,
		EventLeaderLaunched:      true,
		EventDeploymentTriggered: true,
		EventBuildFailed:         true,
	}

	for _, ev := range events {
		if !expected[ev] {
			t.Errorf("unexpected event type: %s", ev)
		}
		delete(expected, ev)
	}

	if len(expected) > 0 {
		t.Errorf("missing event types: %v", expected)
	}
}

func TestDefaultRetry(t *testing.T) {
	retry := DefaultRetry()
	if retry.MaxAttempts != 3 {
		t.Errorf("expected max_attempts 3, got %d", retry.MaxAttempts)
	}
	if retry.BackoffSeconds != 5 {
		t.Errorf("expected backoff_seconds 5, got %d", retry.BackoffSeconds)
	}
}
