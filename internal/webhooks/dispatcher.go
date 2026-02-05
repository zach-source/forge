package webhooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Dispatcher handles sending webhook notifications.
type Dispatcher struct {
	config *Config
	client *http.Client
}

// NewDispatcher creates a new webhook dispatcher.
// Returns nil if config is nil or has no webhooks.
func NewDispatcher(cfg *Config) *Dispatcher {
	if cfg == nil || len(cfg.Webhooks) == 0 {
		return nil
	}
	return &Dispatcher{
		config: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Dispatch sends an event to all matching webhook endpoints.
// This method is safe to call on a nil dispatcher.
func (d *Dispatcher) Dispatch(ctx context.Context, event Event) {
	if d == nil || d.config == nil {
		return
	}

	for _, endpoint := range d.config.EnabledEndpoints() {
		if endpoint.ShouldNotify(event.Type, event.Priority, event.WorkerName, event.LeaderRole) {
			go d.sendWithRetry(ctx, endpoint, event)
		}
	}
}

// sendWithRetry sends an event to an endpoint with retry logic.
func (d *Dispatcher) sendWithRetry(ctx context.Context, endpoint Endpoint, event Event) {
	payload := event.ToPayload()
	data, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("⚠️  Webhook %s: failed to marshal payload: %v\n", endpoint.Name, err)
		return
	}

	maxAttempts := endpoint.Retry.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	backoff := time.Duration(endpoint.Retry.BackoffSeconds) * time.Second
	if backoff <= 0 {
		backoff = 5 * time.Second
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return
		}

		err := d.send(ctx, endpoint, data)
		if err == nil {
			return
		}

		lastErr = err

		if attempt < maxAttempts {
			// Exponential backoff
			sleepDuration := backoff * time.Duration(attempt)
			select {
			case <-ctx.Done():
				return
			case <-time.After(sleepDuration):
			}
		}
	}

	fmt.Printf("⚠️  Webhook %s: failed after %d attempts: %v\n", endpoint.Name, maxAttempts, lastErr)
}

// send performs a single webhook delivery attempt.
func (d *Dispatcher) send(ctx context.Context, endpoint Endpoint, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.URL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "foundry-supervisor/1.0")

	// Add custom headers
	for k, v := range endpoint.Headers {
		req.Header.Set(k, v)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	// Read body for error messages
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return fmt.Errorf("http %d: %s", resp.StatusCode, string(body))
}

// DispatchSync sends an event and waits for completion (for testing).
func (d *Dispatcher) DispatchSync(ctx context.Context, event Event) error {
	if d == nil || d.config == nil {
		return nil
	}

	var lastErr error
	for _, endpoint := range d.config.EnabledEndpoints() {
		if endpoint.ShouldNotify(event.Type, event.Priority, event.WorkerName, event.LeaderRole) {
			payload := event.ToPayload()
			data, err := json.Marshal(payload)
			if err != nil {
				lastErr = err
				continue
			}

			if err := d.send(ctx, endpoint, data); err != nil {
				lastErr = err
			}
		}
	}
	return lastErr
}
