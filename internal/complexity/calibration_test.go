package complexity

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "complexity-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestTracker_RecordAndSave(t *testing.T) {
	dir := setupTestDir(t)

	tracker, err := NewTracker(dir)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}

	tracker.RecordEstimate("task-1", ComplexityMedium)
	tracker.RecordEstimate("task-2", ComplexityLarge)

	if err := tracker.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file exists
	path := filepath.Join(dir, ".forge", "complexity", "calibration.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("calibration file not created")
	}

	// Reload and verify
	tracker2, err := NewTracker(dir)
	if err != nil {
		t.Fatalf("NewTracker reload: %v", err)
	}

	records := tracker2.Records()
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	if records[0].TaskID != "task-1" || records[0].Estimated != ComplexityMedium {
		t.Errorf("record 0: got %+v", records[0])
	}
	if records[1].TaskID != "task-2" || records[1].Estimated != ComplexityLarge {
		t.Errorf("record 1: got %+v", records[1])
	}
}

func TestTracker_RecordCompletion(t *testing.T) {
	dir := setupTestDir(t)

	tracker, err := NewTracker(dir)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}

	tracker.RecordEstimate("task-1", ComplexityMedium)
	tracker.RecordStart("task-1")
	tracker.RecordCompletion("task-1", ComplexityLarge)

	records := tracker.Records()
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	r := records[0]
	if r.Actual != ComplexityLarge {
		t.Errorf("actual: got %s, want L", r.Actual)
	}
	if r.CompletedAt.IsZero() {
		t.Error("expected CompletedAt to be set")
	}
	if r.StartedAt.IsZero() {
		t.Error("expected StartedAt to be set")
	}
}

func TestTracker_UpdateExisting(t *testing.T) {
	dir := setupTestDir(t)

	tracker, err := NewTracker(dir)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}

	tracker.RecordEstimate("task-1", ComplexitySmall)
	tracker.RecordEstimate("task-1", ComplexityMedium) // update

	records := tracker.Records()
	if len(records) != 1 {
		t.Fatalf("expected 1 record (updated), got %d", len(records))
	}
	if records[0].Estimated != ComplexityMedium {
		t.Errorf("expected M after update, got %s", records[0].Estimated)
	}
}

func TestTracker_Accuracy(t *testing.T) {
	dir := setupTestDir(t)

	tracker, err := NewTracker(dir)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}

	// No records
	if acc := tracker.Accuracy(); acc != -1 {
		t.Errorf("expected -1 for no records, got %f", acc)
	}

	// Add records
	tracker.RecordEstimate("task-1", ComplexityMedium)
	tracker.RecordCompletion("task-1", ComplexityMedium) // correct

	tracker.RecordEstimate("task-2", ComplexitySmall)
	tracker.RecordCompletion("task-2", ComplexityLarge) // wrong

	// 1/2 = 0.5
	acc := tracker.Accuracy()
	if acc != 0.5 {
		t.Errorf("expected 0.5 accuracy, got %f", acc)
	}
}

func TestTracker_EmptyDir(t *testing.T) {
	dir := setupTestDir(t)

	tracker, err := NewTracker(dir)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}

	if len(tracker.Records()) != 0 {
		t.Error("expected no records for new tracker")
	}
}

func TestEstimatedMinutes(t *testing.T) {
	tests := []struct {
		c    Complexity
		want int
	}{
		{ComplexitySmall, 30},
		{ComplexityMedium, 60},
		{ComplexityLarge, 120},
		{ComplexityXL, 240},
	}

	for _, tt := range tests {
		if got := estimatedMinutes(tt.c); got != tt.want {
			t.Errorf("estimatedMinutes(%s) = %d, want %d", tt.c, got, tt.want)
		}
	}
}
