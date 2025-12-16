package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/costa-app/costa-cli/pkg/version"
)

var (
	statusFormat string
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Costa CLI status",
	Long:  `Display the current version and build information of the Costa CLI.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if statusFormat == "json" {
			return outputStatusJSON(cmd)
		}
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Costa CLI %s\n", version.Get())
		fmt.Fprintf(out, "Build: %s\n", version.Commit)
		fmt.Fprintf(out, "Date: %s\n", version.Date)
		return nil
	},
}

func outputStatusJSON(cmd *cobra.Command) error {
	output := map[string]string{
		"version": version.Get(),
		"build":   version.Commit,
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
	statusCmd.Flags().StringVar(&statusFormat, "format", "", "Output format (json)")
}
