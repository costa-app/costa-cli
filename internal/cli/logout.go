package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/costa-app/costa-cli/internal/auth"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout from Costa",
	Long:  `Remove your authentication token and logout from Costa.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if logged in
		if !auth.IsLoggedIn() {
			fmt.Fprintln(cmd.OutOrStdout(), "Not currently logged in.")
			return nil
		}

		// Delete token
		if err := auth.DeleteToken(); err != nil {
			return fmt.Errorf("failed to logout: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Successfully logged out!")
		return nil
	},
}
