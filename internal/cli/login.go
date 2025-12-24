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
	"os"
	"os/exec"
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
	loginServerMode    bool   // Internal flag: run as background OAuth server
	loginState         string // Internal: PKCE state for server-mode
	loginVerifier      string // Internal: PKCE verifier for server-mode
	loginWaitTimeout   = 10 * time.Minute // Proposed reasonable wait window
	pollInterval       = 500 * time.Millisecond
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Costa",
	Long:  `Login to Costa using OAuth2 to obtain an access token.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// If running in background server mode, just run the OAuth server
		if loginServerMode {
			return runOAuthServer(cmd)
		}

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

		// JSON mode: spawn fresh background server with this invocation's PKCE params
		if loginFormat == "json" {
			// Generate PKCE parameters for this session
			state, err := generateRandomState()
			if err != nil {
				return fmt.Errorf("failed to generate state: %w", err)
			}
			verifier, err := generateCodeVerifier()
			if err != nil {
				return fmt.Errorf("failed to generate code verifier: %w", err)
			}
			challenge := codeChallengeS256(verifier)

			// Kill any existing server on the port
			if err := shutdownExistingServer(); err != nil {
				// Non-fatal, continue anyway
			}

			// Start fresh background OAuth server with PKCE params
			executable, err := os.Executable()
			if err != nil {
				return fmt.Errorf("failed to get executable path: %w", err)
			}

			bgCmd := exec.Command(executable, "login", "--server-mode",
				"--state", state,
				"--verifier", verifier)
			bgCmd.Stdout = nil
			bgCmd.Stderr = nil
			bgCmd.Stdin = nil

			// Detach from parent process
			bgCmd.SysProcAttr = &syscall.SysProcAttr{
				Setpgid: true,
			}

			if err := bgCmd.Start(); err != nil {
				return fmt.Errorf("failed to start background server: %w", err)
			}

			// Detach the process
			_ = bgCmd.Process.Release()

			// Wait a moment for server to start
			time.Sleep(200 * time.Millisecond)

			// Build auth URL with this session's challenge
			config := auth.OAuthConfig()
			authURL := config.AuthCodeURL(state,
				oauth2.AccessTypeOffline,
				oauth2.SetAuthURLParam("code_challenge", challenge),
				oauth2.SetAuthURLParam("code_challenge_method", "S256"),
			)

			// Return immediately with auth URL
			return writeJSON(cmd, map[string]any{
				"status":          "waiting_for_user",
				"auth_url":        authURL,
				"timeout_seconds": int(loginWaitTimeout / time.Second),
				"redirect_uri":    auth.GetRedirectURL(),
				"message":         "OAuth server started in background, poll 'costa status --format json' to detect completion",
			})
		}

		// Interactive mode - run the OAuth flow synchronously
		return runInteractiveLogin(cmd)
	},
}

// runOAuthServer runs the OAuth callback server in background mode
func runOAuthServer(cmd *cobra.Command) error {
	config := auth.OAuthConfig()

	// Validate we have state and verifier
	if loginState == "" || loginVerifier == "" {
		return fmt.Errorf("server-mode requires --state and --verifier flags")
	}

	// Listen on the callback port
	ln, err := net.Listen("tcp", ":"+auth.RedirectPort)
	if err != nil {
		return fmt.Errorf("failed to bind callback port: %w", err)
	}
	defer func() { _ = ln.Close() }()

	// Channel for receiving OAuth code
	codeChan := make(chan string)
	shutdownChan := make(chan struct{})

	mux := http.NewServeMux()

	// Shutdown endpoint (for graceful shutdown)
	mux.HandleFunc("/costa-code-cli/shutdown", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("shutting down"))
		go func() {
			time.Sleep(100 * time.Millisecond)
			close(shutdownChan)
		}()
	})

	// OAuth callback endpoint
	mux.HandleFunc("/costa-code-cli/callback", func(w http.ResponseWriter, r *http.Request) {
		receivedState := r.URL.Query().Get("state")
		code := r.URL.Query().Get("code")

		// Verify state matches
		if receivedState != loginState {
			http.Error(w, "Invalid state parameter", http.StatusBadRequest)
			return
		}

		if code == "" {
			http.Error(w, "No authorization code received", http.StatusBadRequest)
			return
		}

		// Send success page
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, loginSuccessHTML)

		// Send code to main goroutine
		go func() {
			codeChan <- code
		}()
	})

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Start server in background
	go func() {
		_ = server.Serve(ln)
	}()

	// Wait for callback, shutdown signal, or timeout
	var code string
	select {
	case code = <-codeChan:
		// Continue with token exchange
	case <-shutdownChan:
		_ = server.Shutdown(context.Background())
		return nil // Graceful shutdown
	case <-time.After(loginWaitTimeout):
		_ = server.Shutdown(context.Background())
		return nil // Silent timeout
	}

	// Shutdown server
	_ = server.Shutdown(context.Background())

	// Exchange authorization code for token with PKCE verifier
	token, err := config.Exchange(context.Background(), code,
		oauth2.SetAuthURLParam("code_verifier", loginVerifier),
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, _ = auth.GetCodingToken(ctx)
	// Ignore errors in background mode

	return nil
}

// shutdownExistingServer attempts to gracefully shutdown any server on the OAuth port
func shutdownExistingServer() error {
	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest("GET", "http://127.0.0.1:"+auth.RedirectPort+"/costa-code-cli/shutdown", nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		// Server might not exist or not be ours, that's ok
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	// Wait a bit for server to shut down
	time.Sleep(300 * time.Millisecond)
	return nil
}

// runInteractiveLogin handles the interactive OAuth flow
func runInteractiveLogin(cmd *cobra.Command) error {
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

	// Try to listen
	ln, err := net.Listen("tcp", ":"+auth.RedirectPort)
	if err != nil {
		if errors.Is(err, syscall.EADDRINUSE) {
			// Another process is listening, wait for login
			config := auth.OAuthConfig()
			authURL := config.AuthCodeURL(state,
				oauth2.AccessTypeOffline,
				oauth2.SetAuthURLParam("code_challenge", challenge),
				oauth2.SetAuthURLParam("code_challenge_method", "S256"),
			)
			fmt.Fprintln(cmd.OutOrStdout(), "Opening browser for authentication...")
			fmt.Fprintf(cmd.OutOrStdout(), "\nIf the browser doesn't open automatically, visit:\n%s\n\n", authURL)
			_ = openBrowser(authURL)

			ctx, cancel := context.WithTimeout(context.Background(), loginWaitTimeout)
			defer cancel()
			if err := waitUntilLoggedIn(ctx); err != nil {
				return fmt.Errorf("authentication timeout - please try again: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Successfully logged in!")
			return nil
		}
		return fmt.Errorf("failed to bind callback port: %w", err)
	}

	// Start server in goroutine
	go func() {
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("callback server error: %w", err)
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

	// Try to open browser
	_ = openBrowser(authURL)

	// Wait for callback or error
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

	fmt.Fprintln(cmd.OutOrStdout(), "Successfully logged in!")
	return nil
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
	loginCmd.Flags().BoolVar(&loginServerMode, "server-mode", false, "(internal) Run OAuth server in background mode")
	loginCmd.Flags().StringVar(&loginState, "state", "", "(internal) PKCE state for server mode")
	loginCmd.Flags().StringVar(&loginVerifier, "verifier", "", "(internal) PKCE verifier for server mode")

	// Hide internal flags
	_ = loginCmd.Flags().MarkHidden("server-mode")
	_ = loginCmd.Flags().MarkHidden("state")
	_ = loginCmd.Flags().MarkHidden("verifier")
}
