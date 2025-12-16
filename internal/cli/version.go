package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/costa-app/costa-cli/pkg/version"
)

var (
	longVersion   bool
	versionFormat string
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of costa",
	RunE: func(cmd *cobra.Command, args []string) error {
		if versionFormat == "json" {
			return outputVersionJSON(cmd)
		}
		// Print full version by default (matches tests)
		fmt.Fprintln(cmd.OutOrStdout(), version.GetFull())
		return nil
	},
}

func outputVersionJSON(cmd *cobra.Command) error {
	output := map[string]string{
		"version": version.Get(),
		"commit":  version.Commit,
		"date":    version.Date,
	}

	data, err := json.Marshal(output)
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

func init() {
	versionCmd.Flags().BoolVarP(&longVersion, "long", "l", false, "Show full version with commit and build date")
	versionCmd.Flags().StringVar(&versionFormat, "format", "", "Output format (json)")
}
