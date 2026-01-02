package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  "Print the version, commit hash, and build date of graft.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "graft %s\n", Version)
		fmt.Fprintf(cmd.OutOrStdout(), "  commit: %s\n", Commit)
		fmt.Fprintf(cmd.OutOrStdout(), "  built:  %s\n", Date)
	},
}
