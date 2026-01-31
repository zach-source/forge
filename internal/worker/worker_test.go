package worker

import (
	"testing"
)

func TestRole(t *testing.T) {
	tests := []struct {
		role             Role
		wantStr          string
		wantSingleThread bool
	}{
		{RoleWorker, "worker", false},
		{RolePlanner, "planner", false},
		{RoleReviewer, "reviewer", false},
		{RoleMerge, "merge", true},
		{RoleDeploy, "deploy", true},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			if got := tt.role.String(); got != tt.wantStr {
				t.Errorf("Role.String() = %q, want %q", got, tt.wantStr)
			}
			if got := tt.role.IsSingleThreaded(); got != tt.wantSingleThread {
				t.Errorf("Role.IsSingleThreaded() = %v, want %v", got, tt.wantSingleThread)
			}
		})
	}
}

func TestParseRole(t *testing.T) {
	tests := []struct {
		input   string
		want    Role
		wantErr bool
	}{
		{"worker", RoleWorker, false},
		{"planner", RolePlanner, false},
		{"reviewer", RoleReviewer, false},
		{"merge", RoleMerge, false},
		{"deploy", RoleDeploy, false},
		{"invalid", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseRole(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRole() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseRole() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStatus(t *testing.T) {
	tests := []struct {
		status      Status
		wantStr     string
		wantAvail   bool
		wantRunning bool
	}{
		{StatusIdle, "idle", true, false},
		{StatusActive, "active", false, true},
		{StatusPaused, "paused", false, true},
		{StatusStopped, "stopped", true, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.String(); got != tt.wantStr {
				t.Errorf("Status.String() = %q, want %q", got, tt.wantStr)
			}
			if got := tt.status.IsAvailable(); got != tt.wantAvail {
				t.Errorf("Status.IsAvailable() = %v, want %v", got, tt.wantAvail)
			}
			if got := tt.status.IsRunning(); got != tt.wantRunning {
				t.Errorf("Status.IsRunning() = %v, want %v", got, tt.wantRunning)
			}
		})
	}
}

func TestWorkerDisplayName(t *testing.T) {
	w := &Worker{Name: "alpha"}
	if got := w.DisplayName(); got != "alpha" {
		t.Errorf("DisplayName() = %q, want %q", got, "alpha")
	}

	w.Alias = "api-dev"
	if got := w.DisplayName(); got != "api-dev" {
		t.Errorf("DisplayName() with alias = %q, want %q", got, "api-dev")
	}
}

func TestWorkerShortID(t *testing.T) {
	w := &Worker{ID: "w-a1b2c3d4"}
	if got := w.ShortID(); got != "a1b2c3d4" {
		t.Errorf("ShortID() = %q, want %q", got, "a1b2c3d4")
	}

	w.ID = "short"
	if got := w.ShortID(); got != "short" {
		t.Errorf("ShortID() for short ID = %q, want %q", got, "short")
	}
}

func TestWorkerTmuxSessionName(t *testing.T) {
	w := &Worker{ID: "w-a1b2c3d4", Name: "alpha"}
	if got := w.TmuxSessionName(); got != "forge-alpha-a1b2c3d4" {
		t.Errorf("TmuxSessionName() = %q, want %q", got, "forge-alpha-a1b2c3d4")
	}
}

func TestWorkerGraphitiGroupID(t *testing.T) {
	w := &Worker{ID: "w-a1b2c3d4"}
	if got := w.GraphitiGroupID(); got != "forge-worker-a1b2c3d4" {
		t.Errorf("GraphitiGroupID() = %q, want %q", got, "forge-worker-a1b2c3d4")
	}
}

func TestWorkerValidate(t *testing.T) {
	tests := []struct {
		name    string
		worker  *Worker
		wantErr bool
	}{
		{
			name: "valid",
			worker: &Worker{
				ID:     "w-test",
				Name:   "alpha",
				Role:   RoleWorker,
				Status: StatusIdle,
			},
			wantErr: false,
		},
		{
			name:    "missing ID",
			worker:  &Worker{Name: "alpha", Role: RoleWorker, Status: StatusIdle},
			wantErr: true,
		},
		{
			name:    "missing name",
			worker:  &Worker{ID: "w-test", Role: RoleWorker, Status: StatusIdle},
			wantErr: true,
		},
		{
			name:    "invalid role",
			worker:  &Worker{ID: "w-test", Name: "alpha", Role: "invalid", Status: StatusIdle},
			wantErr: true,
		},
		{
			name:    "invalid status",
			worker:  &Worker{ID: "w-test", Name: "alpha", Role: RoleWorker, Status: "invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.worker.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWorkerClone(t *testing.T) {
	original := &Worker{
		ID:     "w-test",
		Name:   "alpha",
		Alias:  "test-worker",
		Role:   RoleWorker,
		Status: StatusActive,
	}

	clone := original.Clone()

	// Check values are equal
	if clone.ID != original.ID {
		t.Errorf("Clone ID = %q, want %q", clone.ID, original.ID)
	}

	// Modify clone and verify original unchanged
	clone.Name = "bravo"
	if original.Name != "alpha" {
		t.Errorf("Original Name changed after modifying clone")
	}
}
