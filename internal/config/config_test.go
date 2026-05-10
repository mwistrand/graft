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
	if cfg.Model != "" {
		t.Errorf("expected model to be empty, got %q", cfg.Model)
	}
	if cfg.CopilotAPIPackage != DefaultCopilotAPIPackage {
		t.Errorf("expected CopilotAPIPackage %q, got %q", DefaultCopilotAPIPackage, cfg.CopilotAPIPackage)
	}
	if cfg.CopilotAcknowledged {
		t.Error("expected CopilotAcknowledged to default to false")
	}
}

func TestConfigSetGet(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		key   string
		value string
	}{
		{"provider", "copilot"},
		{"model", "gpt-4"},
		{"review-model", "gpt-4o"},
		{"order-model", "gpt-3.5"},
		{"anthropic-api-key", "sk-ant-test123"},
		{"copilot-base-url", "http://localhost:5000"},
		{"copilot-api-package", "copilot-api@1.2.3"},
		{"copilot-acknowledged", "true"},
		{"delta-path", "/usr/local/bin/delta"},
		{"prompt-timeout", "60"},
		{"tests-first", "true"},
		{"inline-tests", "true"},
		{"no-delta", "true"},
		{"no-analyze", "true"},
		{"major-only", "true"},
		{"summarize", "true"},
		{"review-categories", "design,functionality"},
		{"review-severity", "critical"},
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
			if tt.key == "anthropic-api-key" {
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
			name: "valid copilot config",
			cfg: &Config{
				Provider: "copilot",
			},
			wantErr: false,
		},
		{
			name: "openai is no longer supported",
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
	envVars := []string{
		"GRAFT_PROVIDER", "GRAFT_MODEL", "GRAFT_REVIEW_MODEL", "GRAFT_ORDER_MODEL",
		"ANTHROPIC_API_KEY",
		"COPILOT_BASE_URL", "GRAFT_COPILOT_API_PACKAGE", "GRAFT_COPILOT_ACKNOWLEDGED",
		"GRAFT_DELTA_PATH", "GRAFT_PROMPT_TIMEOUT", "GRAFT_TESTS_FIRST",
		"GRAFT_INLINE_TESTS", "GRAFT_NO_DELTA", "GRAFT_NO_ANALYZE", "GRAFT_MAJOR_ONLY",
		"GRAFT_SUMMARIZE", "GRAFT_REVIEW_CATEGORIES", "GRAFT_REVIEW_SEVERITY",
	}
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
	os.Setenv("GRAFT_PROVIDER", "copilot")
	os.Setenv("GRAFT_MODEL", "gpt-4-turbo")
	os.Setenv("GRAFT_REVIEW_MODEL", "gpt-4o")
	os.Setenv("GRAFT_ORDER_MODEL", "gpt-3.5-turbo")
	os.Setenv("ANTHROPIC_API_KEY", "env-anthropic-key")
	os.Setenv("COPILOT_BASE_URL", "http://localhost:5000")
	os.Setenv("GRAFT_COPILOT_API_PACKAGE", "copilot-api@9.9.9")
	os.Setenv("GRAFT_COPILOT_ACKNOWLEDGED", "yes")
	os.Setenv("GRAFT_DELTA_PATH", "/custom/delta")
	os.Setenv("GRAFT_PROMPT_TIMEOUT", "45")
	os.Setenv("GRAFT_TESTS_FIRST", "true")
	os.Setenv("GRAFT_INLINE_TESTS", "1")
	os.Setenv("GRAFT_NO_DELTA", "yes")
	os.Setenv("GRAFT_NO_ANALYZE", "true")
	os.Setenv("GRAFT_MAJOR_ONLY", "true")
	os.Setenv("GRAFT_SUMMARIZE", "true")
	os.Setenv("GRAFT_REVIEW_CATEGORIES", "design,tests")
	os.Setenv("GRAFT_REVIEW_SEVERITY", "suggestion")

	cfg := DefaultConfig()
	cfg.applyEnvOverrides()

	if cfg.Provider != "copilot" {
		t.Errorf("Provider = %q, want %q", cfg.Provider, "copilot")
	}
	if cfg.Model != "gpt-4-turbo" {
		t.Errorf("Model = %q, want %q", cfg.Model, "gpt-4-turbo")
	}
	if cfg.ReviewModel != "gpt-4o" {
		t.Errorf("ReviewModel = %q, want %q", cfg.ReviewModel, "gpt-4o")
	}
	if cfg.OrderModel != "gpt-3.5-turbo" {
		t.Errorf("OrderModel = %q, want %q", cfg.OrderModel, "gpt-3.5-turbo")
	}
	if cfg.AnthropicAPIKey != "env-anthropic-key" {
		t.Errorf("AnthropicAPIKey = %q, want %q", cfg.AnthropicAPIKey, "env-anthropic-key")
	}
	if cfg.CopilotBaseURL != "http://localhost:5000" {
		t.Errorf("CopilotBaseURL = %q, want %q", cfg.CopilotBaseURL, "http://localhost:5000")
	}
	if cfg.CopilotAPIPackage != "copilot-api@9.9.9" {
		t.Errorf("CopilotAPIPackage = %q, want %q", cfg.CopilotAPIPackage, "copilot-api@9.9.9")
	}
	if !cfg.CopilotAcknowledged {
		t.Error("CopilotAcknowledged should be true (parsed from 'yes')")
	}
	if cfg.DeltaPath != "/custom/delta" {
		t.Errorf("DeltaPath = %q, want %q", cfg.DeltaPath, "/custom/delta")
	}
	if cfg.PromptTimeout != 45 {
		t.Errorf("PromptTimeout = %d, want %d", cfg.PromptTimeout, 45)
	}
	if !cfg.TestsFirst {
		t.Error("TestsFirst should be true")
	}
	if !cfg.InlineTests {
		t.Error("InlineTests should be true (parsed from '1')")
	}
	if !cfg.NoDelta {
		t.Error("NoDelta should be true (parsed from 'yes')")
	}
	if !cfg.NoAnalyze {
		t.Error("NoAnalyze should be true")
	}
	if !cfg.MajorOnly {
		t.Error("MajorOnly should be true")
	}
	if !cfg.Summarize {
		t.Error("Summarize should be true")
	}
	if cfg.ReviewCategories != "design,tests" {
		t.Errorf("ReviewCategories = %q, want %q", cfg.ReviewCategories, "design,tests")
	}
	if cfg.ReviewSeverity != "suggestion" {
		t.Errorf("ReviewSeverity = %q, want %q", cfg.ReviewSeverity, "suggestion")
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
		ReviewModel:     "claude-haiku-3-5-20241022",
		OrderModel:      "claude-sonnet-4-20250514",
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
	if loaded.ReviewModel != cfg.ReviewModel {
		t.Errorf("ReviewModel = %q, want %q", loaded.ReviewModel, cfg.ReviewModel)
	}
	if loaded.OrderModel != cfg.OrderModel {
		t.Errorf("OrderModel = %q, want %q", loaded.OrderModel, cfg.OrderModel)
	}
	if loaded.AnthropicAPIKey != cfg.AnthropicAPIKey {
		t.Errorf("AnthropicAPIKey = %q, want %q", loaded.AnthropicAPIKey, cfg.AnthropicAPIKey)
	}
	if loaded.DeltaPath != cfg.DeltaPath {
		t.Errorf("DeltaPath = %q, want %q", loaded.DeltaPath, cfg.DeltaPath)
	}
}

// TestLoadFromExplicitPathMissing verifies that LoadFrom errors when an
// explicitly named path does not exist — callers asked for a specific file
// and silent fallback would mask the mistake.
func TestLoadFromExplicitPathMissing(t *testing.T) {
	tmpDir := t.TempDir()
	missing := filepath.Join(tmpDir, "does-not-exist.json")

	if _, err := LoadFrom(missing); err == nil {
		t.Errorf("LoadFrom(%q) succeeded; want error for missing explicit path", missing)
	}
}

// TestLoadFromDefaultPathMissing verifies that LoadFrom("") silently returns
// the default config when the default config file is absent (first-run
// behavior).
func TestLoadFromDefaultPathMissing(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	for _, v := range []string{"GRAFT_PROVIDER", "GRAFT_MODEL", "ANTHROPIC_API_KEY"} {
		os.Unsetenv(v)
	}

	cfg, err := LoadFrom("")
	if err != nil {
		t.Fatalf("LoadFrom(\"\") with missing default config returned error: %v", err)
	}
	if cfg.Provider != DefaultProvider {
		t.Errorf("Provider = %q, want default %q", cfg.Provider, DefaultProvider)
	}
}

// TestLoadFromExplicitPathExists verifies that LoadFrom reads from a
// non-default path when one is provided.
func TestLoadFromExplicitPathExists(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "alt.json")

	cfg := &Config{
		Provider:        "claude",
		Model:           "claude-sonnet-4-6",
		AnthropicAPIKey: "sk-ant-explicit",
	}
	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo(%q) failed: %v", path, err)
	}

	for _, v := range []string{"GRAFT_PROVIDER", "GRAFT_MODEL", "ANTHROPIC_API_KEY"} {
		os.Unsetenv(v)
	}

	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom(%q) failed: %v", path, err)
	}
	if loaded.Model != cfg.Model {
		t.Errorf("Model = %q, want %q", loaded.Model, cfg.Model)
	}
	if loaded.AnthropicAPIKey != cfg.AnthropicAPIKey {
		t.Errorf("AnthropicAPIKey = %q, want %q", loaded.AnthropicAPIKey, cfg.AnthropicAPIKey)
	}
}

// TestSaveToExplicitPathCreatesParents verifies SaveTo writes to a non-default
// path and creates any missing parent directories.
func TestSaveToExplicitPathCreatesParents(t *testing.T) {
	tmpDir := t.TempDir()
	nested := filepath.Join(tmpDir, "a", "b", "c", "graft.json")

	cfg := &Config{Provider: "claude", AnthropicAPIKey: "sk-ant-x"}
	if err := cfg.SaveTo(nested); err != nil {
		t.Fatalf("SaveTo(%q) failed: %v", nested, err)
	}

	if _, err := os.Stat(nested); err != nil {
		t.Fatalf("expected file at %q, stat error: %v", nested, err)
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
