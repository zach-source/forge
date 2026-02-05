package health

import (
	"fmt"
	"testing"
	"time"
)

func TestDefaultThresholds(t *testing.T) {
	cfg := DefaultThresholds()
	if cfg.CPUCritical != 90.0 {
		t.Errorf("CPUCritical = %v, want 90", cfg.CPUCritical)
	}
	if cfg.CPUDegraded != 80.0 {
		t.Errorf("CPUDegraded = %v, want 80", cfg.CPUDegraded)
	}
	if cfg.MemoryCritical != 90.0 {
		t.Errorf("MemoryCritical = %v, want 90", cfg.MemoryCritical)
	}
	if cfg.MemoryDegraded != 80.0 {
		t.Errorf("MemoryDegraded = %v, want 80", cfg.MemoryDegraded)
	}
	if cfg.FailureThreshold != 0.5 {
		t.Errorf("FailureThreshold = %v, want 0.5", cfg.FailureThreshold)
	}
	if cfg.APIErrorLimit != 5 {
		t.Errorf("APIErrorLimit = %v, want 5", cfg.APIErrorLimit)
	}
}

func TestFailureTracker(t *testing.T) {
	t.Run("empty tracker returns 0", func(t *testing.T) {
		ft := NewFailureTracker(10)
		if rate := ft.FailureRate(); rate != 0 {
			t.Errorf("FailureRate() = %v, want 0", rate)
		}
	})

	t.Run("all successes", func(t *testing.T) {
		ft := NewFailureTracker(5)
		for i := 0; i < 5; i++ {
			ft.Record(true)
		}
		if rate := ft.FailureRate(); rate != 0 {
			t.Errorf("FailureRate() = %v, want 0", rate)
		}
	})

	t.Run("all failures", func(t *testing.T) {
		ft := NewFailureTracker(5)
		for i := 0; i < 5; i++ {
			ft.Record(false)
		}
		if rate := ft.FailureRate(); rate != 1.0 {
			t.Errorf("FailureRate() = %v, want 1.0", rate)
		}
	})

	t.Run("mixed outcomes", func(t *testing.T) {
		ft := NewFailureTracker(10)
		for i := 0; i < 3; i++ {
			ft.Record(false)
		}
		for i := 0; i < 7; i++ {
			ft.Record(true)
		}
		rate := ft.FailureRate()
		if rate != 0.3 {
			t.Errorf("FailureRate() = %v, want 0.3", rate)
		}
	})

	t.Run("ring buffer wraps", func(t *testing.T) {
		ft := NewFailureTracker(3)
		// Fill buffer with failures
		ft.Record(false)
		ft.Record(false)
		ft.Record(false)
		// Overwrite with successes
		ft.Record(true)
		ft.Record(true)
		ft.Record(true)
		if rate := ft.FailureRate(); rate != 0 {
			t.Errorf("FailureRate() = %v, want 0 after overwrite", rate)
		}
	})

	t.Run("default size", func(t *testing.T) {
		ft := NewFailureTracker(0)
		if ft.size != 10 {
			t.Errorf("size = %d, want 10 for zero input", ft.size)
		}
	})
}

func TestAPIErrorTracker(t *testing.T) {
	t.Run("empty tracker returns 0", func(t *testing.T) {
		at := NewAPIErrorTracker(5 * time.Minute)
		if c := at.Count(); c != 0 {
			t.Errorf("Count() = %d, want 0", c)
		}
	})

	t.Run("counts recent errors", func(t *testing.T) {
		at := NewAPIErrorTracker(5 * time.Minute)
		at.Record()
		at.Record()
		at.Record()
		if c := at.Count(); c != 3 {
			t.Errorf("Count() = %d, want 3", c)
		}
	})

	t.Run("prune removes old errors", func(t *testing.T) {
		at := NewAPIErrorTracker(1 * time.Millisecond)
		at.Record()
		time.Sleep(5 * time.Millisecond)
		at.Prune()
		if c := at.Count(); c != 0 {
			t.Errorf("Count() = %d after prune, want 0", c)
		}
	})

	t.Run("default window", func(t *testing.T) {
		at := NewAPIErrorTracker(0)
		if at.window != 5*time.Minute {
			t.Errorf("window = %v, want 5m for zero input", at.window)
		}
	})
}

func TestCheck(t *testing.T) {
	cfg := DefaultThresholds()

	t.Run("good health", func(t *testing.T) {
		mockResources := func() (float64, float64, error) {
			return 50.0, 60.0, nil
		}
		m, err := Check(cfg, nil, nil, mockResources)
		if err != nil {
			t.Fatal(err)
		}
		if m.Status != StatusGood {
			t.Errorf("Status = %v, want good", m.Status)
		}
		if m.CPUPercent != 50.0 {
			t.Errorf("CPUPercent = %v, want 50", m.CPUPercent)
		}
	})

	t.Run("CPU degraded", func(t *testing.T) {
		mockResources := func() (float64, float64, error) {
			return 85.0, 50.0, nil
		}
		m, err := Check(cfg, nil, nil, mockResources)
		if err != nil {
			t.Fatal(err)
		}
		if m.Status != StatusDegraded {
			t.Errorf("Status = %v, want degraded", m.Status)
		}
	})

	t.Run("CPU critical", func(t *testing.T) {
		mockResources := func() (float64, float64, error) {
			return 95.0, 50.0, nil
		}
		m, err := Check(cfg, nil, nil, mockResources)
		if err != nil {
			t.Fatal(err)
		}
		if m.Status != StatusCritical {
			t.Errorf("Status = %v, want critical", m.Status)
		}
	})

	t.Run("memory degraded", func(t *testing.T) {
		mockResources := func() (float64, float64, error) {
			return 50.0, 85.0, nil
		}
		m, err := Check(cfg, nil, nil, mockResources)
		if err != nil {
			t.Fatal(err)
		}
		if m.Status != StatusDegraded {
			t.Errorf("Status = %v, want degraded", m.Status)
		}
	})

	t.Run("memory critical", func(t *testing.T) {
		mockResources := func() (float64, float64, error) {
			return 50.0, 95.0, nil
		}
		m, err := Check(cfg, nil, nil, mockResources)
		if err != nil {
			t.Fatal(err)
		}
		if m.Status != StatusCritical {
			t.Errorf("Status = %v, want critical", m.Status)
		}
	})

	t.Run("high failure rate is critical", func(t *testing.T) {
		ft := NewFailureTracker(10)
		for i := 0; i < 8; i++ {
			ft.Record(false)
		}
		for i := 0; i < 2; i++ {
			ft.Record(true)
		}
		mockResources := func() (float64, float64, error) {
			return 50.0, 50.0, nil
		}
		m, err := Check(cfg, ft, nil, mockResources)
		if err != nil {
			t.Fatal(err)
		}
		if m.Status != StatusCritical {
			t.Errorf("Status = %v, want critical", m.Status)
		}
		if m.FailureRate != 0.8 {
			t.Errorf("FailureRate = %v, want 0.8", m.FailureRate)
		}
	})

	t.Run("API errors trigger critical", func(t *testing.T) {
		at := NewAPIErrorTracker(5 * time.Minute)
		for i := 0; i < 6; i++ {
			at.Record()
		}
		mockResources := func() (float64, float64, error) {
			return 50.0, 50.0, nil
		}
		m, err := Check(cfg, nil, at, mockResources)
		if err != nil {
			t.Fatal(err)
		}
		if m.Status != StatusCritical {
			t.Errorf("Status = %v, want critical", m.Status)
		}
		if m.APIErrors != 6 {
			t.Errorf("APIErrors = %d, want 6", m.APIErrors)
		}
	})

	t.Run("resource error propagated", func(t *testing.T) {
		mockResources := func() (float64, float64, error) {
			return 0, 0, fmt.Errorf("sensor failed")
		}
		_, err := Check(cfg, nil, nil, mockResources)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("nil resources skips resource check", func(t *testing.T) {
		m, err := Check(cfg, nil, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if m.Status != StatusGood {
			t.Errorf("Status = %v, want good", m.Status)
		}
	})
}
