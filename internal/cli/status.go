package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/costa-app/costa-cli/internal/auth"
)

var (
	statusFormat string
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Costa CLI status",
	Long:  `Display the current login status and usage information.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if statusFormat == "json" {
			return outputStatusJSON(cmd)
		}
		out := cmd.OutOrStdout()

		// Check login status
		loggedIn := auth.IsLoggedIn()
		if loggedIn {
			fmt.Fprintf(out, "Logged in: yes\n")
		} else {
			fmt.Fprintf(out, "Logged in: no\n")
			return nil
		}

		// Fetch usage info asynchronously if logged in
		usageChan := make(chan *UsageInfo)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			usage, _ := fetchUsage(ctx)
			usageChan <- usage
		}()

		// Wait for usage info with timeout
		select {
		case usage := <-usageChan:
			if usage != nil {
				fmt.Fprintf(out, "Usage: %s / %s points\n", formatPoints(usage.Points), usage.TotalPoints)
			}
		case <-time.After(5 * time.Second):
			// Timeout - just continue without usage info
		}

		return nil
	},
}

func outputStatusJSON(cmd *cobra.Command) error {
	loggedIn := auth.IsLoggedIn()
	output := map[string]interface{}{
		"logged_in": loggedIn,
	}

	if loggedIn {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		usage, err := fetchUsage(ctx)
		if err == nil && usage != nil {
			output["points"] = usage.Points
			output["total_points"] = usage.TotalPoints
		}
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

// UsageInfo represents the usage data from /api/v1/usage
type UsageInfo struct {
	Points       float64 `json:"points"`
	TotalPoints  string  `json:"total_points"`
	ContextLen   float64 `json:"context_length"`
	UpdatedAt    string  `json:"updated_at"`
}

// fetchUsage fetches usage information from the Costa API
func fetchUsage(ctx context.Context) (*UsageInfo, error) {
	// Ensure OAuth token is valid
	oauthToken, err := auth.EnsureOAuthTokenValid(ctx)
	if err != nil {
		return nil, err
	}

	// Make request to usage endpoint
	usageURL := auth.GetBaseURL() + "/api/v1/usage"
	req, err := http.NewRequestWithContext(ctx, "GET", usageURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", oauthToken.AccessToken))
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch usage: HTTP %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var usage UsageInfo
	if err := json.Unmarshal(bodyBytes, &usage); err != nil {
		return nil, err
	}

	return &usage, nil
}

// formatPoints formats a points value for display
func formatPoints(points float64) string {
	if points == float64(int(points)) {
		return fmt.Sprintf("%d", int(points))
	}
	return fmt.Sprintf("%.1f", points)
}
