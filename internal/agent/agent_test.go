package agent

import (
	"testing"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr error
	}{
		{
			name:    "missing prompt",
			config:  Config{CompletionPromise: "DONE", WorkDir: "/tmp"},
			wantErr: ErrNoPrompt,
		},
		{
			name:    "missing promise",
			config:  Config{Prompt: "test", WorkDir: "/tmp"},
			wantErr: ErrNoPromise,
		},
		{
			name:    "missing workdir",
			config:  Config{Prompt: "test", CompletionPromise: "DONE"},
			wantErr: ErrNoWorkDir,
		},
		{
			name:    "valid config",
			config:  Config{Prompt: "test", CompletionPromise: "DONE", WorkDir: "/tmp"},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if err != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxIterations != 50 {
		t.Errorf("MaxIterations = %d, want 50", cfg.MaxIterations)
	}
	if !cfg.SkipPermissions {
		t.Error("Expected SkipPermissions to be true by default")
	}
	if len(cfg.MCPServers) == 0 {
		t.Error("Expected default MCP servers")
	}
}
