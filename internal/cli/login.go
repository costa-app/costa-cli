package cli

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/oauth2"

	"github.com/costa-app/costa-cli/internal/auth"
)

//go:embed login_success.html
var loginSuccessHTML string

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Costa",
	Long:  `Login to Costa using OAuth2 to obtain an access token.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if already logged in
		if auth.IsLoggedIn() {
			fmt.Fprintln(cmd.OutOrStdout(), "Already logged in. Use 'costa logout' to logout first.")
			return nil
		}

		// Generate random state for CSRF protection
		state, err := generateRandomState()
		if err != nil {
			return fmt.Errorf("failed to generate state: %w", err)
		}

		// Generate PKCE code verifier and challenge
		verifier, err := generateCodeVerifier()
		if err != nil {
			return fmt.Errorf("failed to generate code verifier: %w", err)
		}
		challenge := codeChallengeS256(verifier)

		// Configure OAuth2
		config := auth.OAuthConfig()

		// Create channel to receive the authorization code
		codeChan := make(chan string)
		errChan := make(chan error)

		// Start local HTTP server to handle callback
		server := &http.Server{
			Addr:              ":" + auth.RedirectPort,
			ReadHeaderTimeout: 10 * time.Second,
		}
		http.HandleFunc("/costa-code-cli/callback", func(w http.ResponseWriter, r *http.Request) {
			// Verify state
			if r.URL.Query().Get("state") != state {
				errChan <- fmt.Errorf("invalid state parameter")
				http.Error(w, "Invalid state parameter", http.StatusBadRequest)
				return
			}

			code := r.URL.Query().Get("code")
			if code == "" {
				errChan <- fmt.Errorf("no authorization code received")
				http.Error(w, "No authorization code received", http.StatusBadRequest)
				return
			}

			// Send success response to browser
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, loginSuccessHTML)

			codeChan <- code
		})

		// Start server in goroutine
		go func() {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				errChan <- fmt.Errorf("failed to start callback server: %w", err)
			}
		}()

		// Build authorization URL with PKCE
		authURL := config.AuthCodeURL(state,
			oauth2.AccessTypeOffline,
			oauth2.SetAuthURLParam("code_challenge", challenge),
			oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		)

		fmt.Fprintln(cmd.OutOrStdout(), "Opening browser for authentication...")
		fmt.Fprintf(cmd.OutOrStdout(), "\nIf the browser doesn't open automatically, visit:\n%s\n\n", authURL)

		// Try to open browser (this will fail gracefully if not possible)
		_ = openBrowser(authURL)

		// Wait for callback or error
		var code string
		select {
		case code = <-codeChan:
			// Success - continue with token exchange
		case err := <-errChan:
			_ = server.Shutdown(context.Background())
			return err
		case <-time.After(5 * time.Minute):
			_ = server.Shutdown(context.Background())
			return fmt.Errorf("authentication timeout - please try again")
		}

		// Shutdown server
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)

		// Exchange authorization code for token with PKCE verifier
		token, err := config.Exchange(context.Background(), code,
			oauth2.SetAuthURLParam("code_verifier", verifier),
		)
		if err != nil {
			return fmt.Errorf("failed to exchange code for token: %w", err)
		}

		// Calculate expiry time if available
		var expiresAt *time.Time
		if !token.Expiry.IsZero() {
			expiresAt = &token.Expiry
		}

		// Save OAuth token
		authToken := &auth.Token{
			OAuth: &auth.TokenData{
				AccessToken:  token.AccessToken,
				RefreshToken: token.RefreshToken,
				TokenType:    token.TokenType,
				ExpiresAt:    expiresAt,
			},
		}

		if err := auth.SaveToken(authToken); err != nil {
			return fmt.Errorf("failed to save token: %w", err)
		}

		// Fetch coding token after OAuth exchange
		ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		_, err = auth.GetCodingToken(ctx)
		if err != nil {
			// Don't fail login if coding token fetch fails, just warn
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to fetch coding token: %v\n", err)
			fmt.Fprintln(cmd.ErrOrStderr(), "You can retry by running any command that requires authentication.")
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Successfully logged in!")
		return nil
	},
}

// generateRandomState generates a random state string for CSRF protection
func generateRandomState() (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// generateCodeVerifier generates a random PKCE code verifier
func generateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// codeChallengeS256 generates a PKCE code challenge from a verifier using S256 method
func codeChallengeS256(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// openBrowser attempts to open the URL in the default browser
func openBrowser(url string) error {
	// This is a simple implementation - you might want to use a library
	// or implement platform-specific logic
	return nil
}
