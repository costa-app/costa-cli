package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/costa-app/costa-cli/internal/auth"
	"github.com/costa-app/costa-cli/internal/debug"
)

var (
	tokenRaw          bool
	tokenIncludeOAuth bool
	tokenFormat       string
)

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Display authentication token",
	Long:  `Display the current authentication token. Use --raw to show the full token.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if logged in
		if !auth.IsLoggedIn() {
			if tokenFormat == "json" {
				output := map[string]interface{}{
					"logged_in": false,
				}
				data, _ := json.Marshal(output)
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Not logged in")
			return nil
		}

		// Load token
		token, err := auth.LoadToken()
		if err != nil {
			return fmt.Errorf("failed to load token: %w", err)
		}

		// Ensure coding token is present/valid
		if _, err := auth.GetCodingToken(cmd.Context()); err != nil {
			// If we can't get the coding token, show a warning but continue
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to fetch coding token: %v\n", err)
		} else {
			// Reload token to get the updated CLI token
			if updated, err2 := auth.LoadToken(); err2 == nil {
				token = updated
			}
		}

		// JSON output
		if tokenFormat == "json" {
			return outputJSON(cmd, token)
		}

		// Human-readable output
		return outputHuman(cmd, token)
	},
}

func outputJSON(cmd *cobra.Command, token *auth.Token) error {
	output := map[string]interface{}{
		"logged_in": true,
	}

	// Add coding token
	if token.Coding != nil {
		codingData := map[string]interface{}{
			"token_type": token.Coding.TokenType,
		}
		if tokenRaw {
			codingData["access_token"] = token.Coding.AccessToken
			if token.Coding.RefreshToken != "" {
				codingData["refresh_token"] = token.Coding.RefreshToken
			}
		} else {
			codingData["access_token"] = redactToken(token.Coding.AccessToken)
		}
		if token.Coding.ExpiresAt != nil {
			codingData["expires_at"] = token.Coding.ExpiresAt.Format("2006-01-02T15:04:05Z07:00")
		}
		output["coding"] = codingData
	}

	// Add OAuth token only if --include-oauth and COSTA_DEBUG=1
	if tokenIncludeOAuth && debug.IsEnabled() && token.OAuth != nil {
		oauthData := map[string]interface{}{
			"token_type": token.OAuth.TokenType,
		}
		if tokenRaw {
			oauthData["access_token"] = token.OAuth.AccessToken
			if token.OAuth.RefreshToken != "" {
				oauthData["refresh_token"] = token.OAuth.RefreshToken
			}
		} else {
			oauthData["access_token"] = redactToken(token.OAuth.AccessToken)
		}
		if token.OAuth.ExpiresAt != nil {
			oauthData["expires_at"] = token.OAuth.ExpiresAt.Format("2006-01-02T15:04:05Z07:00")
		}
		output["oauth"] = oauthData
	}

	data, err := json.Marshal(output)
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

func outputHuman(cmd *cobra.Command, token *auth.Token) error {
	out := cmd.OutOrStdout()

	fmt.Fprintln(out, "Logged in: yes")
	fmt.Fprintln(out, "")

	// Show coding token
	if token.Coding != nil {
		fmt.Fprintln(out, "Coding Token:")
		fmt.Fprintf(out, "  Type: %s\n", token.Coding.TokenType)
		if tokenRaw {
			fmt.Fprintf(out, "  Access Token: %s\n", token.Coding.AccessToken)
			if token.Coding.RefreshToken != "" {
				fmt.Fprintf(out, "  Refresh Token: %s\n", token.Coding.RefreshToken)
			}
		} else {
			fmt.Fprintf(out, "  Access Token: %s\n", redactToken(token.Coding.AccessToken))
		}
		if token.Coding.ExpiresAt != nil {
			fmt.Fprintf(out, "  Expires: %s\n", token.Coding.ExpiresAt.Format("2006-01-02 15:04:05 MST"))
		}
		fmt.Fprintln(out, "")
	}

	// Show OAuth token only if --include-oauth and COSTA_DEBUG=1
	if tokenIncludeOAuth && debug.IsEnabled() && token.OAuth != nil {
		fmt.Fprintln(out, "OAuth Token (debug):")
		fmt.Fprintf(out, "  Type: %s\n", token.OAuth.TokenType)
		if tokenRaw {
			fmt.Fprintf(out, "  Access Token: %s\n", token.OAuth.AccessToken)
			if token.OAuth.RefreshToken != "" {
				fmt.Fprintf(out, "  Refresh Token: %s\n", token.OAuth.RefreshToken)
			}
		} else {
			fmt.Fprintf(out, "  Access Token: %s\n", redactToken(token.OAuth.AccessToken))
		}
		if token.OAuth.ExpiresAt != nil {
			fmt.Fprintf(out, "  Expires: %s\n", token.OAuth.ExpiresAt.Format("2006-01-02 15:04:05 MST"))
		}
		fmt.Fprintln(out, "")
	}

	// Show hint if no coding token yet
	if token.Coding == nil && token.OAuth != nil {
		fmt.Fprintln(out, "Note: OAuth token obtained, but no coding token yet.")
		fmt.Fprintln(out, "      Coding token exchange will be implemented soon.")
	}

	if !tokenRaw {
		fmt.Fprintln(out, "Use --raw to show full token")
	}

	return nil
}

// redactToken redacts a token to show only first 6 and last 4 characters
func redactToken(token string) string {
	if len(token) <= 10 {
		return "****"
	}
	return token[:6] + "****" + token[len(token)-4:]
}

func init() {
	tokenCmd.Flags().BoolVar(&tokenRaw, "raw", false, "Show full token (use with caution)")
	tokenCmd.Flags().StringVar(&tokenFormat, "format", "", "Output format (json)")

	// Only show --include-oauth flag if COSTA_DEBUG is enabled
	oauthFlag := tokenCmd.Flags().VarPF(
		newBoolValue(&tokenIncludeOAuth),
		"include-oauth",
		"",
		"Include OAuth token (requires COSTA_DEBUG=1)",
	)
	if !debug.IsEnabled() {
		oauthFlag.Hidden = true
	}
}

// boolValue implements pflag.Value for bool
type boolValue bool

func newBoolValue(p *bool) *boolValue {
	return (*boolValue)(p)
}

func (b *boolValue) Set(s string) error {
	v := s == "true" || s == "1" || s == "t" || s == "T" || s == "TRUE" || s == "True"
	*b = boolValue(v)
	return nil
}

func (b *boolValue) Type() string {
	return "bool"
}

func (b *boolValue) String() string {
	if *b {
		return "true"
	}
	return "false"
}
