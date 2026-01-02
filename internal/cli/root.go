// Package cli provides the command-line interface for graft.
package cli

import (
	"fmt"
	"os"

	"github.com/mwistrand/graft/internal/config"
	"github.com/spf13/cobra"
)

var (
	// Version is set at build time.
	Version = "dev"

	// Commit is set at build time.
	Commit = "none"

	// Date is set at build time.
	Date = "unknown"
)

var (
	cfgFile string
	verbose bool
	cfg     *config.Config
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "graft",
	Short: "AI-powered code review CLI",
	Long: `Graft is an AI-powered code review CLI that presents diffs in logical order.

Instead of reviewing files alphabetically, Graft uses AI to determine
the optimal review order based on architectural flow - starting with
entry points and progressing through business logic to adapters.

Example:
  graft review main     Review changes against main branch
  graft config set anthropic-api-key <key>   Set your API key`,
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for help and version commands
		if cmd.Name() == "help" || cmd.Name() == "version" {
			return nil
		}

		var err error
		cfg, err = config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		return nil
	},
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// GetConfig returns the loaded configuration. Only valid after command execution starts.
func GetConfig() *config.Config {
	return cfg
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/graft/config.json)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Add subcommands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(configCmd)
}

// SetVersionInfo sets the version information for the CLI.
// This is called from main() with values set at build time.
func SetVersionInfo(version, commit, date string) {
	Version = version
	Commit = commit
	Date = date
}

// IsVerbose returns whether verbose mode is enabled.
func IsVerbose() bool {
	return verbose
}

// Verbose prints a message if verbose mode is enabled.
func Verbose(format string, args ...any) {
	if verbose {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}
