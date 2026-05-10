// Package config provides configuration management for the graft CLI.
package config

const (
	// DefaultProvider is the default AI provider used for code review.
	DefaultProvider = "claude"

	// DefaultCopilotBaseURL is the default URL for the copilot-api proxy.
	DefaultCopilotBaseURL = "http://localhost:4141"

	// DefaultCopilotAPIPackage is the npm spec used when graft auto-launches
	// the copilot-api proxy. Users should pin a specific version via
	// `graft config set copilot-api-package copilot-api@x.y.z` to limit the
	// supply-chain surface; @latest is retained as the default for users who
	// already accept that tradeoff.
	DefaultCopilotAPIPackage = "copilot-api@latest"

	// DefaultConfigDir is the directory name for graft configuration.
	DefaultConfigDir = ".config/graft"

	// DefaultConfigFile is the configuration file name.
	DefaultConfigFile = "config.json"

	// DefaultPromptTimeout is the default timeout in minutes for interactive prompts.
	// Set to 30 minutes. Use 0 to disable timeout.
	DefaultPromptTimeout = 30
)

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	return &Config{
		Provider:          DefaultProvider,
		PromptTimeout:     DefaultPromptTimeout,
		CopilotAPIPackage: DefaultCopilotAPIPackage,
	}
}
