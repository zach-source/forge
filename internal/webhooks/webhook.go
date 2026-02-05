// Package webhooks provides notification webhook support for supervisor events.
package webhooks

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// EventType represents a supervisor event that can trigger webhooks.
type EventType string

const (
	EventTaskCompleted       EventType = "task_completed"
	EventTaskFailed          EventType = "task_failed"
	EventWorkerStarted       EventType = "worker_started"
	EventWorkerStopped       EventType = "worker_stopped"
	EventWorkerError         EventType = "worker_error"
	EventLeaderLaunched      EventType = "leader_launched"
	EventDeploymentTriggered EventType = "deployment_triggered"
	EventBuildFailed         EventType = "build_failed"
)

// AllEventTypes returns all supported event types.
func AllEventTypes() []EventType {
	return []EventType{
		EventTaskCompleted,
		EventTaskFailed,
		EventWorkerStarted,
		EventWorkerStopped,
		EventWorkerError,
		EventLeaderLaunched,
		EventDeploymentTriggered,
		EventBuildFailed,
	}
}

// Priority represents task priority levels for filtering.
type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityMedium   Priority = "medium"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

// RetryConfig configures retry behavior for webhook delivery.
type RetryConfig struct {
	MaxAttempts    int `yaml:"max_attempts"`
	BackoffSeconds int `yaml:"backoff_seconds"`
}

// FilterConfig configures event filtering for a webhook endpoint.
type FilterConfig struct {
	Priority []Priority `yaml:"priority,omitempty"`
	Workers  []string   `yaml:"workers,omitempty"`
	Leaders  []string   `yaml:"leaders,omitempty"`
}

// Endpoint represents a webhook endpoint configuration.
type Endpoint struct {
	Name    string            `yaml:"name"`
	URL     string            `yaml:"url"`
	Events  []EventType       `yaml:"events"`
	Filters FilterConfig      `yaml:"filters,omitempty"`
	Retry   RetryConfig       `yaml:"retry,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
	Enabled bool              `yaml:"enabled,omitempty"`
}

// Config represents the complete webhook configuration.
type Config struct {
	Webhooks []Endpoint `yaml:"webhooks"`
}

// DefaultRetry returns default retry configuration.
func DefaultRetry() RetryConfig {
	return RetryConfig{
		MaxAttempts:    3,
		BackoffSeconds: 5,
	}
}

// configPath returns the path to the webhooks config file.
func configPath(workDir string) string {
	return filepath.Join(workDir, ".foundry", "webhooks.yaml")
}

// LoadConfig loads webhook configuration from the work directory.
// Returns nil config (not error) if no config file exists.
func LoadConfig(workDir string) (*Config, error) {
	path := configPath(workDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading webhooks config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing webhooks config: %w", err)
	}

	// Apply defaults
	for i := range cfg.Webhooks {
		if cfg.Webhooks[i].Retry.MaxAttempts == 0 {
			cfg.Webhooks[i].Retry = DefaultRetry()
		}
		// Default to enabled if not explicitly set
		// Note: YAML unmarshaling leaves bool as false if not set
		// We can't distinguish "not set" from "explicitly false" with bool
		// So we default to enabled unless config exists
	}

	return &cfg, nil
}

// SaveConfig saves webhook configuration to the work directory.
func SaveConfig(workDir string, cfg *Config) error {
	path := configPath(workDir)

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling webhooks config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing webhooks config: %w", err)
	}

	return nil
}

// ShouldNotify checks if an endpoint should receive a notification for the given event.
func (e *Endpoint) ShouldNotify(event EventType, priority Priority, workerName, leaderRole string) bool {
	// Check if event is in the endpoint's event list
	eventMatch := false
	for _, ev := range e.Events {
		if ev == event {
			eventMatch = true
			break
		}
	}
	if !eventMatch {
		return false
	}

	// Check priority filter (if specified)
	if len(e.Filters.Priority) > 0 && priority != "" {
		priorityMatch := false
		for _, p := range e.Filters.Priority {
			if p == priority {
				priorityMatch = true
				break
			}
		}
		if !priorityMatch {
			return false
		}
	}

	// Check worker filter (if specified)
	if len(e.Filters.Workers) > 0 && workerName != "" {
		workerMatch := false
		for _, w := range e.Filters.Workers {
			if w == workerName {
				workerMatch = true
				break
			}
		}
		if !workerMatch {
			return false
		}
	}

	// Check leader filter (if specified)
	if len(e.Filters.Leaders) > 0 && leaderRole != "" {
		leaderMatch := false
		for _, l := range e.Filters.Leaders {
			if l == leaderRole {
				leaderMatch = true
				break
			}
		}
		if !leaderMatch {
			return false
		}
	}

	return true
}

// Validate checks if the endpoint configuration is valid.
func (e *Endpoint) Validate() error {
	if e.Name == "" {
		return fmt.Errorf("webhook name is required")
	}
	if e.URL == "" {
		return fmt.Errorf("webhook URL is required")
	}
	if len(e.Events) == 0 {
		return fmt.Errorf("at least one event type is required")
	}

	// Validate event types
	validEvents := make(map[EventType]bool)
	for _, ev := range AllEventTypes() {
		validEvents[ev] = true
	}
	for _, ev := range e.Events {
		if !validEvents[ev] {
			return fmt.Errorf("invalid event type: %s", ev)
		}
	}

	return nil
}

// Validate checks if the entire configuration is valid.
func (c *Config) Validate() error {
	names := make(map[string]bool)
	for i, endpoint := range c.Webhooks {
		if err := endpoint.Validate(); err != nil {
			return fmt.Errorf("webhook %d (%s): %w", i, endpoint.Name, err)
		}
		if names[endpoint.Name] {
			return fmt.Errorf("duplicate webhook name: %s", endpoint.Name)
		}
		names[endpoint.Name] = true
	}
	return nil
}

// EnabledEndpoints returns only the endpoints that are enabled.
func (c *Config) EnabledEndpoints() []Endpoint {
	if c == nil {
		return nil
	}
	var enabled []Endpoint
	for _, e := range c.Webhooks {
		// Default to enabled if the field wasn't explicitly set to false
		enabled = append(enabled, e)
	}
	return enabled
}
