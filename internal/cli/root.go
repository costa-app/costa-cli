package cli

import (
	"github.com/spf13/cobra"

	"github.com/costa-app/costa-cli/pkg/version"
)

var rootCmd = &cobra.Command{
	Use:   "costa",
	Short: "Costa CLI is the best way to build with AI",
	Long:  `Costa CLI helps you install plugins and manage your account.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Enable global --version flag
	rootCmd.Version = version.Get()
	rootCmd.SetVersionTemplate("{{.Version}}\n")

	// Disable command sorting, so we can control order
	cobra.EnableCommandSorting = false

	// Add subcommands
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
	rootCmd.AddCommand(tokenCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(setupCmd)
}
