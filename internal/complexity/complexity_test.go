package complexity

import (
	"testing"
)

func TestComplexityValid(t *testing.T) {
	tests := []struct {
		c    Complexity
		want bool
	}{
		{ComplexitySmall, true},
		{ComplexityMedium, true},
		{ComplexityLarge, true},
		{ComplexityXL, true},
		{"", false},
		{"XXL", false},
		{"invalid", false},
	}

	for _, tt := range tests {
		if got := tt.c.Valid(); got != tt.want {
			t.Errorf("Complexity(%q).Valid() = %v, want %v", tt.c, got, tt.want)
		}
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		input   string
		want    Complexity
		wantErr bool
	}{
		{"S", ComplexitySmall, false},
		{"s", ComplexitySmall, false},
		{"small", ComplexitySmall, false},
		{"M", ComplexityMedium, false},
		{"medium", ComplexityMedium, false},
		{"L", ComplexityLarge, false},
		{"large", ComplexityLarge, false},
		{"XL", ComplexityXL, false},
		{"xl", ComplexityXL, false},
		{"  M  ", ComplexityMedium, false},
		{"", "", true},
		{"XXL", "", true},
		{"invalid", "", true},
	}

	for _, tt := range tests {
		got, err := Parse(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("Parse(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("Parse(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestScore(t *testing.T) {
	tests := []struct {
		c    Complexity
		want int
	}{
		{ComplexitySmall, 1},
		{ComplexityMedium, 2},
		{ComplexityLarge, 3},
		{ComplexityXL, 4},
		{"", 2},    // default to medium
		{"XXL", 2}, // default to medium
	}

	for _, tt := range tests {
		if got := Score(tt.c); got != tt.want {
			t.Errorf("Score(%q) = %d, want %d", tt.c, got, tt.want)
		}
	}
}

func TestEstimateFromText(t *testing.T) {
	tests := []struct {
		name        string
		title       string
		description string
		wantSize    Complexity
	}{
		{
			name:        "simple typo fix",
			title:       "Fix typo in README",
			description: "There's a typo in the README file",
			wantSize:    ComplexitySmall,
		},
		{
			name:  "moderate feature",
			title: "Add new API endpoint",
			description: `Add a new endpoint for user preferences. Needs handler, validation, and database query.

Files to modify:
- internal/api/handler.go
- internal/api/routes.go
- internal/db/preferences.go

## Acceptance Criteria
- GET /api/preferences returns user settings
- PUT /api/preferences updates settings`,
			wantSize: ComplexityMedium,
		},
		{
			name:  "complex refactor",
			title: "Refactor authentication system",
			description: `Refactor the authentication system to support multiple providers.
This requires changes to:
- internal/auth/provider.go
- internal/auth/oauth.go
- internal/auth/jwt.go
- internal/auth/middleware.go
- internal/auth/store.go
- cmd/server/auth.go
- cmd/server/config.go

## Acceptance Criteria
- Support OAuth2 providers
- Backward compatible with existing JWT tokens
- All existing tests pass`,
			wantSize: ComplexityXL,
		},
		{
			name:        "migration task",
			title:       "Migrate database schema",
			description: "Need to migrate the user table schema to support new fields",
			wantSize:    ComplexityMedium, // at least M due to "migrate" keyword
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			est := EstimateFromText(tt.title, tt.description)
			if est.Size != tt.wantSize {
				t.Errorf("EstimateFromText() size = %s, want %s (factors: %v, score mapped)", est.Size, tt.wantSize, est.Factors)
			}
			if len(est.Factors) == 0 {
				t.Error("expected at least one factor")
			}
		})
	}
}

func TestEstimateFromText_FileCount(t *testing.T) {
	est := EstimateFromText("Update configs", `Changes needed:
- config.yaml
- internal/config/loader.go
- internal/config/parser.go
- cmd/main.go
- Makefile
- docker-compose.yml`)

	if est.FileCount < 5 {
		t.Errorf("FileCount = %d, want >= 5", est.FileCount)
	}
}

func TestCountFileHints(t *testing.T) {
	desc := `Files to modify:
- internal/auth/handler.go
- internal/auth/store.go
- cmd/server/main.go
- config.yaml
- This line has no files`

	count := countFileHints(desc)
	if count != 4 {
		t.Errorf("countFileHints() = %d, want 4", count)
	}
}
