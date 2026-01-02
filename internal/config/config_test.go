package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Provider != DefaultProvider {
		t.Errorf("expected provider %q, got %q", DefaultProvider, cfg.Provider)
	}
	if cfg.Model != DefaultModel {
		t.Errorf("expected model %q, got %q", DefaultModel, cfg.Model)
	}
}

func TestConfigSetGet(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		key   string
		value string
	}{
		{"provider", "openai"},
		{"model", "gpt-4"},
		{"anthropic-api-key", "sk-ant-test123"},
		{"openai-api-key", "sk-test456"},
		{"copilot-base-url", "http://localhost:5000"},
		{"delta-path", "/usr/local/bin/delta"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if err := cfg.Set(tt.key, tt.value); err != nil {
				t.Fatalf("Set(%q, %q) failed: %v", tt.key, tt.value, err)
			}

			got, err := cfg.Get(tt.key)
			if err != nil {
				t.Fatalf("Get(%q) failed: %v", tt.key, err)
			}

			// API keys are masked on Get
			if tt.key == "anthropic-api-key" || tt.key == "openai-api-key" {
				if got == tt.value {
					t.Error("expected API key to be masked")
				}
			} else {
				if got != tt.value {
					t.Errorf("Get(%q) = %q, want %q", tt.key, got, tt.value)
				}
			}
		})
	}
}

func TestConfigSetUnknownKey(t *testing.T) {
	cfg := DefaultConfig()
	err := cfg.Set("unknown-key", "value")
	if err == nil {
		t.Error("expected error for unknown key")
	}
}

func TestConfigGetUnknownKey(t *testing.T) {
	cfg := DefaultConfig()
	_, err := cfg.Get("unknown-key")
	if err == nil {
		t.Error("expected error for unknown key")
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "valid claude config",
			cfg: &Config{
				Provider:        "claude",
				AnthropicAPIKey: "sk-ant-test",
			},
			wantErr: false,
		},
		{
			name: "claude without api key",
			cfg: &Config{
				Provider: "claude",
			},
			wantErr: true,
		},
		{
			name: "valid openai config",
			cfg: &Config{
				Provider:     "openai",
				OpenAIAPIKey: "sk-test",
			},
			wantErr: false,
		},
		{
			name: "valid copilot config",
			cfg: &Config{
				Provider: "copilot",
			},
			wantErr: false,
		},
		{
			name: "openai without api key",
			cfg: &Config{
				Provider: "openai",
			},
			wantErr: true,
		},
		{
			name: "unknown provider",
			cfg: &Config{
				Provider: "unknown",
			},
			wantErr: true,
		},
		{
			name: "empty provider",
			cfg: &Config{
				Provider: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigEnvOverrides(t *testing.T) {
	// Save and restore environment
	envVars := []string{"GRAFT_PROVIDER", "GRAFT_MODEL", "ANTHROPIC_API_KEY", "OPENAI_API_KEY", "COPILOT_BASE_URL", "GRAFT_DELTA_PATH"}
	saved := make(map[string]string)
	for _, v := range envVars {
		saved[v] = os.Getenv(v)
	}
	defer func() {
		for k, v := range saved {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	// Set test environment
	os.Setenv("GRAFT_PROVIDER", "openai")
	os.Setenv("GRAFT_MODEL", "gpt-4-turbo")
	os.Setenv("ANTHROPIC_API_KEY", "env-anthropic-key")
	os.Setenv("OPENAI_API_KEY", "env-openai-key")
	os.Setenv("COPILOT_BASE_URL", "http://localhost:5000")
	os.Setenv("GRAFT_DELTA_PATH", "/custom/delta")

	cfg := DefaultConfig()
	cfg.applyEnvOverrides()

	if cfg.Provider != "openai" {
		t.Errorf("Provider = %q, want %q", cfg.Provider, "openai")
	}
	if cfg.Model != "gpt-4-turbo" {
		t.Errorf("Model = %q, want %q", cfg.Model, "gpt-4-turbo")
	}
	if cfg.AnthropicAPIKey != "env-anthropic-key" {
		t.Errorf("AnthropicAPIKey = %q, want %q", cfg.AnthropicAPIKey, "env-anthropic-key")
	}
	if cfg.OpenAIAPIKey != "env-openai-key" {
		t.Errorf("OpenAIAPIKey = %q, want %q", cfg.OpenAIAPIKey, "env-openai-key")
	}
	if cfg.CopilotBaseURL != "http://localhost:5000" {
		t.Errorf("CopilotBaseURL = %q, want %q", cfg.CopilotBaseURL, "http://localhost:5000")
	}
	if cfg.DeltaPath != "/custom/delta" {
		t.Errorf("DeltaPath = %q, want %q", cfg.DeltaPath, "/custom/delta")
	}
}

func TestConfigSaveLoad(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()

	// Override home directory for the test
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Clear env vars that would override file config
	for _, v := range []string{"GRAFT_PROVIDER", "GRAFT_MODEL", "ANTHROPIC_API_KEY"} {
		os.Unsetenv(v)
	}

	// Create and save a config
	cfg := &Config{
		Provider:        "claude",
		Model:           "claude-opus-4-20250514",
		AnthropicAPIKey: "test-api-key",
		DeltaPath:       "/usr/bin/delta",
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify file exists
	configPath := filepath.Join(tmpDir, DefaultConfigDir, DefaultConfigFile)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	// Load and verify
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if loaded.Provider != cfg.Provider {
		t.Errorf("Provider = %q, want %q", loaded.Provider, cfg.Provider)
	}
	if loaded.Model != cfg.Model {
		t.Errorf("Model = %q, want %q", loaded.Model, cfg.Model)
	}
	if loaded.AnthropicAPIKey != cfg.AnthropicAPIKey {
		t.Errorf("AnthropicAPIKey = %q, want %q", loaded.AnthropicAPIKey, cfg.AnthropicAPIKey)
	}
	if loaded.DeltaPath != cfg.DeltaPath {
		t.Errorf("DeltaPath = %q, want %q", loaded.DeltaPath, cfg.DeltaPath)
	}
}

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"", "****"},
		{"short", "****"},
		{"12345678", "****"},
		{"123456789", "1234...6789"},
		{"sk-ant-api-xxxxxxxxxxxxxxxxxxxxx", "sk-a...xxxx"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := maskAPIKey(tt.key)
			if got != tt.want {
				t.Errorf("maskAPIKey(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}
