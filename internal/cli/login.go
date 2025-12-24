package cli

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/oauth2"

	"github.com/costa-app/costa-cli/internal/auth"
)

//go:embed login_success.html
var loginSuccessHTML string

var (
	loginFormat        string
	loginWaitTimeout   = 10 * time.Minute // Proposed reasonable wait window
	pollInterval       = 500 * time.Millisecond
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Costa",
	Long:  `Login to Costa using OAuth2 to obtain an access token.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// If we're already logged in, exit early
		if auth.IsLoggedIn() {
			if loginFormat == "json" {
				return writeJSON(cmd, map[string]any{
					"status":    "already_logged_in",
					"logged_in": true,
				})
			}
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

		mux := http.NewServeMux()
		// Ready endpoint so other invocations can detect our listener
		mux.HandleFunc("/costa-code-cli/ready", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})

		// OAuth callback handler
		mux.HandleFunc("/costa-code-cli/callback", func(w http.ResponseWriter, r *http.Request) {
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

		// Prepare local server
		server := &http.Server{
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
		}

		// Try to acquire the port; if already in use, reuse existing listener
		ln, listenErr := net.Listen("tcp", ":"+auth.RedirectPort)
		existingListener := false
		if listenErr != nil {
			if errors.Is(listenErr, syscall.EADDRINUSE) {
				existingListener = true
			} else {
				return fmt.Errorf("failed to bind callback port: %w", listenErr)
			}
		}

		if !existingListener {
			// Start server in goroutine
			go func() {
				if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
					errChan <- fmt.Errorf("callback server error: %w", err)
				}
			}()
		}

		// Build authorization URL with PKCE
		authURL := config.AuthCodeURL(state,
			oauth2.AccessTypeOffline,
			oauth2.SetAuthURLParam("code_challenge", challenge),
			oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		)

		if loginFormat == "json" {
			_ = writeJSON(cmd, map[string]any{
				"status":                    "waiting_for_user",
				"auth_url":                  authURL,
				"using_existing_listener":   existingListener,
				"timeout_seconds":           int(loginWaitTimeout / time.Second),
				"redirect_uri":              auth.GetRedirectURL(),
			})
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "Opening browser for authentication...")
			fmt.Fprintf(cmd.OutOrStdout(), "\nIf the browser doesn't open automatically, visit:\n%s\n\n", authURL)
		}

		// Try to open browser (this will fail gracefully if not possible)
		_ = openBrowser(authURL)

		// Wait for completion
		if existingListener {
			// Another costa login is already listening on the port.
			// Wait until we detect a successful login (token saved) or timeout.
			ctx, cancel := context.WithTimeout(context.Background(), loginWaitTimeout)
			defer cancel()
			if err := waitUntilLoggedIn(ctx); err != nil {
				return fmt.Errorf("authentication timeout - please try again: %w", err)
			}
			if loginFormat == "json" {
				return writeJSON(cmd, map[string]any{
					"status":    "success",
					"logged_in": true,
				})
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Successfully logged in!")
			return nil
		}

		// Otherwise, handle the callback ourselves
		var code string
		select {
		case code = <-codeChan:
			// continue
		case err := <-errChan:
			_ = server.Shutdown(context.Background())
			return err
		case <-time.After(loginWaitTimeout):
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

		if loginFormat == "json" {
			return writeJSON(cmd, map[string]any{
				"status":    "success",
				"logged_in": true,
			})
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Successfully logged in!")
		return nil
	},
}

// writeJSON prints a single-line JSON object to stdout
func writeJSON(cmd *cobra.Command, m map[string]any) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// waitUntilLoggedIn polls until auth.IsLoggedIn() returns true or context is done
func waitUntilLoggedIn(ctx context.Context) error {
	for {
		if auth.IsLoggedIn() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}
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

func init() {
	loginCmd.Flags().StringVar(&loginFormat, "format", "", "Output format (json)")
}
