package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds all configuration for the graft CLI.
type Config struct {
	// Provider specifies which AI provider to use (e.g., "claude", "copilot").
	Provider string `json:"provider,omitempty"`

	// Model specifies the model to use with the selected provider.
	Model string `json:"model,omitempty"`

	// AnthropicAPIKey is the API key for the Anthropic/Claude provider.
	AnthropicAPIKey string `json:"anthropic_api_key,omitempty"`

	// OpenAIAPIKey is the API key for the OpenAI provider.
	OpenAIAPIKey string `json:"openai_api_key,omitempty"`

	// CopilotBaseURL is the URL of the copilot-api proxy server.
	CopilotBaseURL string `json:"copilot_base_url,omitempty"`

	// DeltaPath is the path to the delta binary. If empty, uses PATH lookup.
	DeltaPath string `json:"delta_path,omitempty"`
}

// Load reads configuration from the default config file and environment variables.
// Environment variables take precedence over file configuration.
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Try to load from config file
	configPath, err := ConfigPath()
	if err != nil {
		return nil, fmt.Errorf("determining config path: %w", err)
	}

	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing config file: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// Environment variables override file configuration
	cfg.applyEnvOverrides()

	return cfg, nil
}

// Save writes the configuration to the default config file.
func (c *Config) Save() error {
	configPath, err := ConfigPath()
	if err != nil {
		return fmt.Errorf("determining config path: %w", err)
	}

	// Ensure config directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// ConfigPath returns the full path to the configuration file.
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, DefaultConfigDir, DefaultConfigFile), nil
}

// Validate checks if the configuration has all required values for the selected provider.
func (c *Config) Validate() error {
	switch c.Provider {
	case "claude", "":
		if c.AnthropicAPIKey == "" {
			return errors.New("anthropic API key not set; run 'graft config set anthropic-api-key <key>' or set ANTHROPIC_API_KEY")
		}
	case "copilot":
		// Copilot requires the copilot-api proxy to be running, no API key needed
		return nil
	case "openai":
		if c.OpenAIAPIKey == "" {
			return errors.New("openai API key not set; run 'graft config set openai-api-key <key>' or set OPENAI_API_KEY")
		}
	default:
		return fmt.Errorf("unknown provider %q; available providers: claude, copilot", c.Provider)
	}
	return nil
}

// applyEnvOverrides applies environment variable overrides to the configuration.
func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("GRAFT_PROVIDER"); v != "" {
		c.Provider = v
	}
	if v := os.Getenv("GRAFT_MODEL"); v != "" {
		c.Model = v
	}
	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
		c.AnthropicAPIKey = v
	}
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		c.OpenAIAPIKey = v
	}
	if v := os.Getenv("COPILOT_BASE_URL"); v != "" {
		c.CopilotBaseURL = v
	}
	if v := os.Getenv("GRAFT_DELTA_PATH"); v != "" {
		c.DeltaPath = v
	}
}

// Set updates a configuration key with the given value.
func (c *Config) Set(key, value string) error {
	switch key {
	case "provider":
		c.Provider = value
	case "model":
		c.Model = value
	case "anthropic-api-key":
		c.AnthropicAPIKey = value
	case "openai-api-key":
		c.OpenAIAPIKey = value
	case "copilot-base-url":
		c.CopilotBaseURL = value
	case "delta-path":
		c.DeltaPath = value
	default:
		return fmt.Errorf("unknown configuration key: %s", key)
	}
	return nil
}

// Get retrieves a configuration value by key.
func (c *Config) Get(key string) (string, error) {
	switch key {
	case "provider":
		return c.Provider, nil
	case "model":
		return c.Model, nil
	case "anthropic-api-key":
		if c.AnthropicAPIKey == "" {
			return "", nil
		}
		return maskAPIKey(c.AnthropicAPIKey), nil
	case "openai-api-key":
		if c.OpenAIAPIKey == "" {
			return "", nil
		}
		return maskAPIKey(c.OpenAIAPIKey), nil
	case "copilot-base-url":
		return c.CopilotBaseURL, nil
	case "delta-path":
		return c.DeltaPath, nil
	default:
		return "", fmt.Errorf("unknown configuration key: %s", key)
	}
}

// maskAPIKey returns a masked version of an API key for display.
func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}
