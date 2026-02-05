package health

import (
	"fmt"
	"sync"
	"time"
)

// taskOutcome records a single task result.
type taskOutcome struct {
	success bool
	time    time.Time
}

// FailureTracker tracks recent task outcomes using a ring buffer.
type FailureTracker struct {
	mu       sync.Mutex
	outcomes []taskOutcome
	pos      int
	size     int
}

// NewFailureTracker creates a tracker that remembers the last n outcomes.
func NewFailureTracker(size int) *FailureTracker {
	if size <= 0 {
		size = 10
	}
	return &FailureTracker{
		outcomes: make([]taskOutcome, size),
		size:     size,
	}
}

// Record adds a task outcome (true = success, false = failure).
func (t *FailureTracker) Record(success bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.outcomes[t.pos] = taskOutcome{success: success, time: time.Now()}
	t.pos = (t.pos + 1) % t.size
}

// FailureRate returns the fraction of recent outcomes that were failures.
// Returns 0 if no outcomes have been recorded.
func (t *FailureTracker) FailureRate() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	var failures, total int
	for _, o := range t.outcomes {
		if !o.time.IsZero() {
			total++
			if !o.success {
				failures++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(failures) / float64(total)
}

// apiError records a single API error timestamp.
type apiError struct {
	time time.Time
}

// APIErrorTracker counts API errors within a sliding time window.
type APIErrorTracker struct {
	mu     sync.Mutex
	errors []apiError
	window time.Duration
}

// NewAPIErrorTracker creates a tracker with the given time window.
func NewAPIErrorTracker(window time.Duration) *APIErrorTracker {
	if window <= 0 {
		window = 5 * time.Minute
	}
	return &APIErrorTracker{
		window: window,
	}
}

// Record adds an API error at the current time.
func (a *APIErrorTracker) Record() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.errors = append(a.errors, apiError{time: time.Now()})
}

// Count returns how many errors occurred within the window.
func (a *APIErrorTracker) Count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.countLocked(time.Now())
}

func (a *APIErrorTracker) countLocked(now time.Time) int {
	cutoff := now.Add(-a.window)
	count := 0
	for _, e := range a.errors {
		if e.time.After(cutoff) {
			count++
		}
	}
	return count
}

// Prune removes errors older than the window.
func (a *APIErrorTracker) Prune() {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-a.window)
	kept := a.errors[:0]
	for _, e := range a.errors {
		if e.time.After(cutoff) {
			kept = append(kept, e)
		}
	}
	a.errors = kept
}

// Check evaluates system health against thresholds and returns metrics.
// It combines resource usage, failure rate, and API error count into a
// single health status. If getResources is nil, resource metrics are skipped.
func Check(cfg ThresholdConfig, failures *FailureTracker, apiErrors *APIErrorTracker, getResources func() (float64, float64, error)) (*Metrics, error) {
	m := &Metrics{
		Status: StatusGood,
	}

	// Resource metrics
	if getResources != nil {
		cpuPct, memPct, err := getResources()
		if err != nil {
			return nil, fmt.Errorf("reading resources: %w", err)
		}
		m.CPUPercent = cpuPct
		m.MemoryPercent = memPct

		if cpuPct >= cfg.CPUCritical {
			m.Status = StatusCritical
			m.Reason = fmt.Sprintf("CPU %.0f%% >= %.0f%%", cpuPct, cfg.CPUCritical)
			return m, nil
		}
		if memPct >= cfg.MemoryCritical {
			m.Status = StatusCritical
			m.Reason = fmt.Sprintf("memory %.0f%% >= %.0f%%", memPct, cfg.MemoryCritical)
			return m, nil
		}
		if cpuPct >= cfg.CPUDegraded {
			m.Status = StatusDegraded
			m.Reason = fmt.Sprintf("CPU %.0f%% >= %.0f%%", cpuPct, cfg.CPUDegraded)
		}
		if memPct >= cfg.MemoryDegraded && m.Status != StatusDegraded {
			m.Status = StatusDegraded
			m.Reason = fmt.Sprintf("memory %.0f%% >= %.0f%%", memPct, cfg.MemoryDegraded)
		}
	}

	// Failure rate
	if failures != nil {
		rate := failures.FailureRate()
		m.FailureRate = rate
		if rate >= cfg.FailureThreshold {
			if m.Status != StatusCritical {
				m.Status = StatusCritical
				m.Reason = fmt.Sprintf("failure rate %.0f%% >= %.0f%%", rate*100, cfg.FailureThreshold*100)
			}
			return m, nil
		}
	}

	// API errors
	if apiErrors != nil {
		count := apiErrors.Count()
		m.APIErrors = count
		if count >= cfg.APIErrorLimit {
			if m.Status != StatusCritical {
				m.Status = StatusCritical
				m.Reason = fmt.Sprintf("API errors %d >= %d in window", count, cfg.APIErrorLimit)
			}
			return m, nil
		}
	}

	return m, nil
}
