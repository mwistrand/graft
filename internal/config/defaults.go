// Package config provides configuration management for the graft CLI.
package config

const (
	// DefaultProvider is the default AI provider used for code review.
	DefaultProvider = "claude"

	// DefaultModel is the default Claude model to use.
	DefaultModel = "claude-sonnet-4-20250514"

	// DefaultConfigDir is the directory name for graft configuration.
	DefaultConfigDir = ".config/graft"

	// DefaultConfigFile is the configuration file name.
	DefaultConfigFile = "config.json"
)

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	return &Config{
		Provider: DefaultProvider,
		Model:    DefaultModel,
	}
}
