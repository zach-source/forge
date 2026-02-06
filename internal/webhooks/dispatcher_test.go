package webhooks

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewDispatcher(t *testing.T) {
	// Nil config returns nil dispatcher
	d := NewDispatcher(nil)
	if d != nil {
		t.Error("expected nil dispatcher for nil config")
	}

	// Empty webhooks returns nil dispatcher
	d = NewDispatcher(&Config{Webhooks: []Endpoint{}})
	if d != nil {
		t.Error("expected nil dispatcher for empty webhooks")
	}

	// Valid config returns dispatcher
	cfg := &Config{
		Webhooks: []Endpoint{
			{Name: "test", URL: "http://example.com", Events: []EventType{EventTaskCompleted}},
		},
	}
	d = NewDispatcher(cfg)
	if d == nil {
		t.Error("expected non-nil dispatcher for valid config")
	}
}

func TestDispatcherDispatch(t *testing.T) {
	var received atomic.Int32
	var mu sync.Mutex
	var lastPayload Payload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p Payload
		json.NewDecoder(r.Body).Decode(&p)
		mu.Lock()
		lastPayload = p
		mu.Unlock()
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{
		Webhooks: []Endpoint{
			{
				Name:   "test",
				URL:    server.URL,
				Events: []EventType{EventTaskCompleted, EventWorkerStarted},
				Retry:  RetryConfig{MaxAttempts: 1, BackoffSeconds: 1},
			},
		},
	}

	d := NewDispatcher(cfg)
	event := NewTaskCompletedEvent("task-123", "Fix bug", "alpha", PriorityHigh, 15)

	// Dispatch and wait a bit for async processing
	d.Dispatch(context.Background(), event)
	time.Sleep(100 * time.Millisecond)

	if received.Load() != 1 {
		t.Errorf("expected 1 request, got %d", received.Load())
	}
	mu.Lock()
	eventType := lastPayload.Event
	mu.Unlock()
	if eventType != EventTaskCompleted {
		t.Errorf("expected event type %s, got %s", EventTaskCompleted, eventType)
	}
}

func TestDispatcherDispatchFiltering(t *testing.T) {
	var received atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{
		Webhooks: []Endpoint{
			{
				Name:   "test",
				URL:    server.URL,
				Events: []EventType{EventTaskCompleted}, // Only task_completed
				Retry:  RetryConfig{MaxAttempts: 1, BackoffSeconds: 1},
			},
		},
	}

	d := NewDispatcher(cfg)

	// This event should be filtered out
	event := NewWorkerStartedEvent("w-123", "alpha", "task-456", "Fix bug", "/path")
	d.Dispatch(context.Background(), event)
	time.Sleep(100 * time.Millisecond)

	if received.Load() != 0 {
		t.Errorf("expected 0 requests (filtered), got %d", received.Load())
	}

	// This event should go through
	event2 := NewTaskCompletedEvent("task-123", "Fix bug", "alpha", PriorityHigh, 15)
	d.Dispatch(context.Background(), event2)
	time.Sleep(100 * time.Millisecond)

	if received.Load() != 1 {
		t.Errorf("expected 1 request, got %d", received.Load())
	}
}

func TestDispatcherDispatchPriorityFiltering(t *testing.T) {
	var received atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{
		Webhooks: []Endpoint{
			{
				Name:   "test",
				URL:    server.URL,
				Events: []EventType{EventTaskCompleted},
				Filters: FilterConfig{
					Priority: []Priority{PriorityHigh, PriorityCritical},
				},
				Retry: RetryConfig{MaxAttempts: 1, BackoffSeconds: 1},
			},
		},
	}

	d := NewDispatcher(cfg)

	// Low priority should be filtered
	event := NewTaskCompletedEvent("task-123", "Fix bug", "alpha", PriorityLow, 15)
	d.Dispatch(context.Background(), event)
	time.Sleep(100 * time.Millisecond)

	if received.Load() != 0 {
		t.Errorf("expected 0 requests (filtered by priority), got %d", received.Load())
	}

	// High priority should go through
	event2 := NewTaskCompletedEvent("task-456", "Critical fix", "alpha", PriorityHigh, 5)
	d.Dispatch(context.Background(), event2)
	time.Sleep(100 * time.Millisecond)

	if received.Load() != 1 {
		t.Errorf("expected 1 request, got %d", received.Load())
	}
}

func TestDispatcherRetry(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := attempts.Add(1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{
		Webhooks: []Endpoint{
			{
				Name:   "test",
				URL:    server.URL,
				Events: []EventType{EventTaskCompleted},
				Retry:  RetryConfig{MaxAttempts: 3, BackoffSeconds: 1}, // 1 second backoff
			},
		},
	}

	d := NewDispatcher(cfg)
	event := NewTaskCompletedEvent("task-123", "Fix bug", "alpha", PriorityHigh, 15)

	// Use DispatchSync for synchronous retry behavior in test
	// This should fail twice (500 error) and succeed on third attempt
	_ = d.DispatchSync(context.Background(), event)

	// The sync dispatch only makes one attempt without retry
	// To test retry, we need to wait for async dispatch
	attempts.Store(0)
	d.Dispatch(context.Background(), event)

	// Wait for retries (1s backoff * 2 attempts + some buffer)
	time.Sleep(3 * time.Second)

	if attempts.Load() < 2 {
		t.Errorf("expected at least 2 attempts, got %d", attempts.Load())
	}
}

func TestDispatcherDispatchSync(t *testing.T) {
	var received atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{
		Webhooks: []Endpoint{
			{
				Name:   "test",
				URL:    server.URL,
				Events: []EventType{EventTaskCompleted},
				Retry:  RetryConfig{MaxAttempts: 1, BackoffSeconds: 1},
			},
		},
	}

	d := NewDispatcher(cfg)
	event := NewTaskCompletedEvent("task-123", "Fix bug", "alpha", PriorityHigh, 15)

	err := d.DispatchSync(context.Background(), event)
	if err != nil {
		t.Errorf("DispatchSync failed: %v", err)
	}

	if received.Load() != 1 {
		t.Errorf("expected 1 request, got %d", received.Load())
	}
}

func TestDispatcherNilSafe(t *testing.T) {
	var d *Dispatcher

	// Should not panic
	d.Dispatch(context.Background(), Event{})
	err := d.DispatchSync(context.Background(), Event{})
	if err != nil {
		t.Errorf("DispatchSync on nil should not error: %v", err)
	}
}

func TestDispatcherHeaders(t *testing.T) {
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{
		Webhooks: []Endpoint{
			{
				Name:   "test",
				URL:    server.URL,
				Events: []EventType{EventTaskCompleted},
				Headers: map[string]string{
					"X-Custom-Header": "custom-value",
					"Authorization":   "Bearer token123",
				},
				Retry: RetryConfig{MaxAttempts: 1, BackoffSeconds: 1},
			},
		},
	}

	d := NewDispatcher(cfg)
	event := NewTaskCompletedEvent("task-123", "Fix bug", "alpha", PriorityHigh, 15)

	d.DispatchSync(context.Background(), event)

	if receivedHeaders.Get("X-Custom-Header") != "custom-value" {
		t.Errorf("expected X-Custom-Header 'custom-value', got '%s'", receivedHeaders.Get("X-Custom-Header"))
	}
	if receivedHeaders.Get("Authorization") != "Bearer token123" {
		t.Errorf("expected Authorization 'Bearer token123', got '%s'", receivedHeaders.Get("Authorization"))
	}
	if receivedHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", receivedHeaders.Get("Content-Type"))
	}
	if receivedHeaders.Get("User-Agent") != "foundry-supervisor/1.0" {
		t.Errorf("expected User-Agent 'foundry-supervisor/1.0', got '%s'", receivedHeaders.Get("User-Agent"))
	}
}

func TestDispatcherContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // Slow handler
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{
		Webhooks: []Endpoint{
			{
				Name:   "test",
				URL:    server.URL,
				Events: []EventType{EventTaskCompleted},
				Retry:  RetryConfig{MaxAttempts: 1, BackoffSeconds: 1},
			},
		},
	}

	d := NewDispatcher(cfg)
	event := NewTaskCompletedEvent("task-123", "Fix bug", "alpha", PriorityHigh, 15)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := d.DispatchSync(ctx, event)
	if err == nil {
		t.Error("expected error due to context timeout")
	}
}
