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
	"github.com/costa-app/costa-cli/internal/debug"
)

var (
	statusFormat string
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Costa CLI status",
	Long:  `Display the current login status and usage information.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if statusFormat == "claude-code" {
			return outputStatusClaudeCode(cmd)
		}
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
			usage, _ := fetchUsageWithCache(ctx)
			usageChan <- usage
		}()

		// Wait for usage info with timeout
		select {
		case usage := <-usageChan:
			if usage != nil {
				pointsStr := "-"
				if usage.Points.IsValid {
					pointsStr = formatPoints(usage.Points.Value)
				}
				fmt.Fprintf(out, "Usage: %s / %s points\n", pointsStr, usage.TotalPoints)
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
		usage, err := fetchUsageWithCache(ctx)
		if err == nil && usage != nil {
			if usage.Points.IsValid {
				output["points"] = usage.Points.Value
			} else {
				output["points"] = "-"
			}
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

func outputStatusClaudeCode(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()

	// Check login status
	if !auth.IsLoggedIn() {
		fmt.Fprintf(out, "Costa: Not logged in")
		return nil
	}

	// Fetch usage with cache
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	usage, err := fetchUsageWithCache(ctx)
	if err != nil || usage == nil {
		fmt.Fprintf(out, "Costa: Error fetching usage")
		return nil
	}

	// Format: "Costa: X / Y points"
	pointsStr := "-"
	if usage.Points.IsValid {
		pointsStr = formatPoints(usage.Points.Value)
	}
	fmt.Fprintf(out, "💫  %s / %s ", pointsStr, usage.TotalPoints)
	return nil
}

func init() {
	statusCmd.Flags().StringVar(&statusFormat, "format", "", "Output format (json|claude-code)")
}

// FlexibleFloat handles JSON fields that can be either a number or a string like "-"
type FlexibleFloat struct {
	Value   float64
	IsValid bool
}

// UnmarshalJSON implements custom unmarshaling for FlexibleFloat
func (f *FlexibleFloat) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as float64 first
	var num float64
	if err := json.Unmarshal(data, &num); err == nil {
		f.Value = num
		f.IsValid = true
		return nil
	}

	// Try to unmarshal as string
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		// If it's a dash or empty, mark as invalid
		if str == "-" || str == "" {
			f.IsValid = false
			return nil
		}
		// Otherwise it's an unexpected string
		return fmt.Errorf("unexpected string value for numeric field: %s", str)
	}

	return fmt.Errorf("cannot unmarshal as number or string")
}

// UsageInfo represents the usage data from /api/v1/usage
type UsageInfo struct {
	TotalPoints string        `json:"total_points"`
	UpdatedAt   string        `json:"updated_at"`
	Points      FlexibleFloat `json:"points"`
	ContextLen  float64       `json:"context_length"`
}

// fetchUsage fetches usage information from the Costa API
func fetchUsage(ctx context.Context) (*UsageInfo, error) {
	debug.Printf("fetchUsage: Starting to fetch usage from API\n")

	// Ensure OAuth token is valid
	oauthToken, err := auth.EnsureOAuthTokenValid(ctx)
	if err != nil {
		debug.Printf("fetchUsage: Failed to get valid OAuth token: %v\n", err)
		return nil, err
	}

	// Make request to usage endpoint
	usageURL := auth.GetBaseURL() + "/api/v1/usage"
	debug.Printf("fetchUsage: Making request to %s\n", usageURL)
	req, err := http.NewRequestWithContext(ctx, "GET", usageURL, nil)
	if err != nil {
		debug.Printf("fetchUsage: Failed to create request: %v\n", err)
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", oauthToken.AccessToken))
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		debug.Printf("fetchUsage: HTTP request failed: %v\n", err)
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	debug.Printf("fetchUsage: Received HTTP %d\n", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch usage: HTTP %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		debug.Printf("fetchUsage: Failed to read response body: %v\n", err)
		return nil, err
	}

	debug.Printf("fetchUsage: Response body: %s\n", string(bodyBytes))

	var usage UsageInfo
	if err := json.Unmarshal(bodyBytes, &usage); err != nil {
		debug.Printf("fetchUsage: Failed to unmarshal JSON: %v\n", err)
		return nil, err
	}

	debug.Printf("fetchUsage: Parsed usage info: %+v\n", usage)

	return &usage, nil
}

// formatPoints formats a points value for display
func formatPoints(points float64) string {
	if points == float64(int(points)) {
		return fmt.Sprintf("%d", int(points))
	}
	return fmt.Sprintf("%.1f", points)
}

// Cache for usage data
type usageCache struct {
	data      *UsageInfo
	timestamp time.Time
}

var globalUsageCache *usageCache

// fetchUsageWithCache fetches usage with 15-second caching
func fetchUsageWithCache(ctx context.Context) (*UsageInfo, error) {
	// Check cache validity (15 seconds)
	if globalUsageCache != nil && time.Since(globalUsageCache.timestamp) < 15*time.Second {
		debug.Printf("Cache hit: returning cached usage (age: %v)\n", time.Since(globalUsageCache.timestamp))
		debug.Printf("Cached data: %+v\n", globalUsageCache.data)
		return globalUsageCache.data, nil
	}

	if globalUsageCache != nil {
		debug.Printf("Cache expired (age: %v), fetching fresh data\n", time.Since(globalUsageCache.timestamp))
	} else {
		debug.Printf("No cache available, fetching fresh data\n")
	}

	// Fetch fresh data
	usage, err := fetchUsage(ctx)
	if err != nil {
		debug.Printf("Error fetching usage: %v\n", err)
		// Return stale cache if available on error
		if globalUsageCache != nil {
			debug.Printf("Returning stale cache due to error\n")
			return globalUsageCache.data, nil
		}
		debug.Printf("No stale cache available, returning error\n")
		return nil, err
	}

	debug.Printf("Successfully fetched usage: %+v\n", usage)

	// Only update cache with successful (non-nil) responses
	if usage != nil {
		debug.Printf("Updating cache with new data\n")
		globalUsageCache = &usageCache{
			data:      usage,
			timestamp: time.Now(),
		}
	} else {
		debug.Printf("Warning: fetchUsage returned nil usage without error - NOT caching\n")
	}

	return usage, nil
}
