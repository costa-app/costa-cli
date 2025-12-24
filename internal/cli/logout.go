package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/costa-app/costa-cli/internal/auth"
)

var (
	logoutFormat string
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout from Costa",
	Long:  `Remove your authentication token and logout from Costa.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if logged in
		if !auth.IsLoggedIn() {
			if logoutFormat == "json" {
				return writeLogoutJSON(cmd, map[string]any{
					"status":    "not_logged_in",
					"logged_in": false,
				})
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Not currently logged in.")
			return nil
		}

		// Delete token
		if err := auth.DeleteToken(); err != nil {
			if logoutFormat == "json" {
				return writeLogoutJSON(cmd, map[string]any{
					"status": "error",
					"error":  err.Error(),
				})
			}
			return fmt.Errorf("failed to logout: %w", err)
		}

		if logoutFormat == "json" {
			return writeLogoutJSON(cmd, map[string]any{
				"status":    "success",
				"logged_in": false,
			})
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Successfully logged out!")
		return nil
	},
}

// writeLogoutJSON prints a single-line JSON object to stdout
func writeLogoutJSON(cmd *cobra.Command, m map[string]any) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

func init() {
	logoutCmd.Flags().StringVar(&logoutFormat, "format", "", "Output format (json)")
}
