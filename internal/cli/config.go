package cli

import (
	"fmt"

	"github.com/mwistrand/graft/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage graft configuration",
	Long: `View and modify graft configuration.

Available keys:
  provider          AI provider to use (claude, openai)
  model             Model name for the selected provider
  anthropic-api-key API key for Claude/Anthropic
  openai-api-key    API key for OpenAI
  delta-path        Path to delta binary`,
	Run: func(cmd *cobra.Command, args []string) {
		// Show current config when run without subcommands
		showConfig()
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		value, err := cfg.Get(args[0])
		if err != nil {
			return err
		}

		if value == "" {
			fmt.Println("(not set)")
		} else {
			fmt.Println(value)
		}
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		if err := cfg.Set(args[0], args[1]); err != nil {
			return err
		}

		if err := cfg.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Printf("Set %s\n", args[0])
		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show the configuration file path",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := config.ConfigPath()
		if err != nil {
			return err
		}
		fmt.Println(path)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configPathCmd)
}

func showConfig() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	fmt.Println("Current configuration:")
	fmt.Println()

	keys := []string{"provider", "model", "anthropic-api-key", "openai-api-key", "delta-path"}
	for _, key := range keys {
		value, _ := cfg.Get(key)
		if value == "" {
			value = "(not set)"
		}
		fmt.Printf("  %-20s %s\n", key+":", value)
	}

	fmt.Println()
	path, _ := config.ConfigPath()
	fmt.Printf("Config file: %s\n", path)
}
