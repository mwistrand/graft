package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds all configuration for the graft CLI.
type Config struct {
	// Provider specifies which AI provider to use (e.g., "claude", "copilot").
	Provider string `json:"provider,omitempty"`

	// Model specifies the default model to use with the selected provider.
	Model string `json:"model,omitempty"`

	// ReviewModel specifies the model to use for review tasks (review, quick review).
	// Falls back to Model if empty.
	ReviewModel string `json:"review_model,omitempty"`

	// OrderModel specifies the model to use for ordering and summary tasks.
	// Falls back to Model if empty.
	OrderModel string `json:"order_model,omitempty"`

	// AnthropicAPIKey is the API key for the Anthropic/Claude provider.
	AnthropicAPIKey string `json:"anthropic_api_key,omitempty"`

	// OpenAIAPIKey is the API key for the OpenAI provider.
	OpenAIAPIKey string `json:"openai_api_key,omitempty"`

	// CopilotBaseURL is the URL of the copilot-api proxy server.
	CopilotBaseURL string `json:"copilot_base_url,omitempty"`

	// DeltaPath is the path to the delta binary. If empty, uses PATH lookup.
	DeltaPath string `json:"delta_path,omitempty"`

	// PromptTimeout is the timeout in minutes for interactive prompts.
	// If the user doesn't respond within this time, the review exits.
	// Set to 0 to disable timeout. Default is 30 minutes.
	PromptTimeout int `json:"prompt_timeout,omitempty"`

	// Review preference fields - these can be set as defaults in config
	// and overridden by CLI flags.

	// TestsFirst shows test files before their implementation files.
	TestsFirst bool `json:"tests_first,omitempty"`

	// InlineTests shows test files alongside their implementation files.
	InlineTests bool `json:"inline_tests,omitempty"`

	// NoDelta disables Delta rendering for diffs.
	NoDelta bool `json:"no_delta,omitempty"`

	// NoAnalyze skips repository structure analysis.
	NoAnalyze bool `json:"no_analyze,omitempty"`

	// MajorOnly only reviews core and supporting groups, skipping minor changes.
	MajorOnly bool `json:"major_only,omitempty"`

	// Summarize includes an AI summary of changes before the review.
	Summarize bool `json:"summarize,omitempty"`

	// ReviewCategories specifies which categories to focus on in AI reviews.
	// Comma-separated list: design,functionality,complexity,tests,naming,comments,style,documentation
	ReviewCategories string `json:"review_categories,omitempty"`

	// ReviewSeverity filters review output by minimum severity level.
	// Valid values: critical, suggestion, nit
	ReviewSeverity string `json:"review_severity,omitempty"`
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
	if v := os.Getenv("GRAFT_REVIEW_MODEL"); v != "" {
		c.ReviewModel = v
	}
	if v := os.Getenv("GRAFT_ORDER_MODEL"); v != "" {
		c.OrderModel = v
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
	if v := os.Getenv("GRAFT_PROMPT_TIMEOUT"); v != "" {
		if timeout, err := strconv.Atoi(v); err == nil {
			c.PromptTimeout = timeout
		}
	}
	// Review preference overrides
	if v := os.Getenv("GRAFT_TESTS_FIRST"); v != "" {
		c.TestsFirst = parseBool(v)
	}
	if v := os.Getenv("GRAFT_INLINE_TESTS"); v != "" {
		c.InlineTests = parseBool(v)
	}
	if v := os.Getenv("GRAFT_NO_DELTA"); v != "" {
		c.NoDelta = parseBool(v)
	}
	if v := os.Getenv("GRAFT_NO_ANALYZE"); v != "" {
		c.NoAnalyze = parseBool(v)
	}
	if v := os.Getenv("GRAFT_MAJOR_ONLY"); v != "" {
		c.MajorOnly = parseBool(v)
	}
	if v := os.Getenv("GRAFT_SUMMARIZE"); v != "" {
		c.Summarize = parseBool(v)
	}
	if v := os.Getenv("GRAFT_REVIEW_CATEGORIES"); v != "" {
		c.ReviewCategories = v
	}
	if v := os.Getenv("GRAFT_REVIEW_SEVERITY"); v != "" {
		c.ReviewSeverity = v
	}
}

// parseBool parses a string as a boolean value.
// Returns true for "true", "1", "yes" (case-insensitive), false otherwise.
func parseBool(s string) bool {
	s = strings.ToLower(s)
	return s == "true" || s == "1" || s == "yes"
}

// Set updates a configuration key with the given value.
func (c *Config) Set(key, value string) error {
	switch key {
	case "provider":
		c.Provider = value
	case "model":
		c.Model = value
	case "review-model":
		c.ReviewModel = value
	case "order-model":
		c.OrderModel = value
	case "anthropic-api-key":
		c.AnthropicAPIKey = value
	case "openai-api-key":
		c.OpenAIAPIKey = value
	case "copilot-base-url":
		c.CopilotBaseURL = value
	case "delta-path":
		c.DeltaPath = value
	case "prompt-timeout":
		timeout, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid prompt-timeout value %q: must be an integer (minutes)", value)
		}
		if timeout < 0 {
			return fmt.Errorf("invalid prompt-timeout value %q: must be >= 0", value)
		}
		c.PromptTimeout = timeout
	case "tests-first":
		c.TestsFirst = parseBool(value)
	case "inline-tests":
		c.InlineTests = parseBool(value)
	case "no-delta":
		c.NoDelta = parseBool(value)
	case "no-analyze":
		c.NoAnalyze = parseBool(value)
	case "major-only":
		c.MajorOnly = parseBool(value)
	case "summarize":
		c.Summarize = parseBool(value)
	case "review-categories":
		c.ReviewCategories = value
	case "review-severity":
		c.ReviewSeverity = value
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
	case "review-model":
		return c.ReviewModel, nil
	case "order-model":
		return c.OrderModel, nil
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
	case "prompt-timeout":
		return strconv.Itoa(c.PromptTimeout), nil
	case "tests-first":
		return strconv.FormatBool(c.TestsFirst), nil
	case "inline-tests":
		return strconv.FormatBool(c.InlineTests), nil
	case "no-delta":
		return strconv.FormatBool(c.NoDelta), nil
	case "no-analyze":
		return strconv.FormatBool(c.NoAnalyze), nil
	case "major-only":
		return strconv.FormatBool(c.MajorOnly), nil
	case "summarize":
		return strconv.FormatBool(c.Summarize), nil
	case "review-categories":
		return c.ReviewCategories, nil
	case "review-severity":
		return c.ReviewSeverity, nil
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
