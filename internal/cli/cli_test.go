package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/costa-app/costa-cli/pkg/version"
)

func TestVersionCommand(t *testing.T) {
	// Save original values
	origVersion := version.Version
	origCommit := version.Commit
	origDate := version.Date

	// Set test values
	version.Version = "1.0.0"
	version.Commit = "abc123"
	version.Date = "2024-01-01"

	// Restore after test
	defer func() {
		version.Version = origVersion
		version.Commit = origCommit
		version.Date = origDate
	}()

	// Capture output
	var buf bytes.Buffer

	// Create a new root command for testing
	testRoot := &cobra.Command{Use: "costa"}
	testRoot.AddCommand(versionCmd)
	testRoot.SetOut(&buf)
	testRoot.SetErr(&buf)
	testRoot.SetArgs([]string{"version"})

	err := testRoot.Execute()
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	output := buf.String()
	expected := "1.0.0 (abc123, 2024-01-01)\n"

	if output != expected {
		t.Errorf("version output mismatch:\nexpected: %q\ngot:      %q", expected, output)
	}
}

func TestStatusCommand(t *testing.T) {
	// Save original values
	origVersion := version.Version
	origCommit := version.Commit
	origDate := version.Date

	// Set test values
	version.Version = "1.0.0"
	version.Commit = "abc123"
	version.Date = "2024-01-01"

	// Restore after test
	defer func() {
		version.Version = origVersion
		version.Commit = origCommit
		version.Date = origDate
	}()

	// Capture output
	var buf bytes.Buffer

	// Create a new root command for testing
	testRoot := &cobra.Command{Use: "costa"}
	testRoot.AddCommand(statusCmd)
	testRoot.SetOut(&buf)
	testRoot.SetErr(&buf)
	testRoot.SetArgs([]string{"status"})

	err := testRoot.Execute()
	if err != nil {
		t.Fatalf("status command failed: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines of output, got %d:\n%s", len(lines), output)
	}

	expectedLines := []string{
		"Costa CLI 1.0.0",
		"Build: abc123",
		"Date: 2024-01-01",
	}

	for i, expected := range expectedLines {
		if lines[i] != expected {
			t.Errorf("line %d mismatch:\nexpected: %q\ngot:      %q", i+1, expected, lines[i])
		}
	}
}

func TestStatusCommandDefaults(t *testing.T) {
	// Test with default dev values
	origVersion := version.Version
	origCommit := version.Commit
	origDate := version.Date

	version.Version = "dev"
	version.Commit = "none"
	version.Date = "unknown"

	defer func() {
		version.Version = origVersion
		version.Commit = origCommit
		version.Date = origDate
	}()

	var buf bytes.Buffer

	// Create a new root command for testing
	testRoot := &cobra.Command{Use: "costa"}
	testRoot.AddCommand(statusCmd)
	testRoot.SetOut(&buf)
	testRoot.SetErr(&buf)
	testRoot.SetArgs([]string{"status"})

	err := testRoot.Execute()
	if err != nil {
		t.Fatalf("status command failed: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "Costa CLI dev") {
		t.Errorf("expected output to contain 'Costa CLI dev', got: %s", output)
	}
	if !strings.Contains(output, "Build: none") {
		t.Errorf("expected output to contain 'Build: none', got: %s", output)
	}
	if !strings.Contains(output, "Date: unknown") {
		t.Errorf("expected output to contain 'Date: unknown', got: %s", output)
	}
}

func TestVersionCommandJSONFormat(t *testing.T) {
	// Save original values
	origVersion := version.Version
	origCommit := version.Commit
	origDate := version.Date

	// Set test values
	version.Version = "1.0.0"
	version.Commit = "abc123"
	version.Date = "2024-01-01"

	// Restore after test
	defer func() {
		version.Version = origVersion
		version.Commit = origCommit
		version.Date = origDate
		versionFormat = "" // Reset flag
	}()

	// Capture output
	var buf bytes.Buffer

	// Create a new root command for testing
	testRoot := &cobra.Command{Use: "costa"}
	testRoot.AddCommand(versionCmd)
	testRoot.SetOut(&buf)
	testRoot.SetErr(&buf)
	testRoot.SetArgs([]string{"version", "--format", "json"})

	err := testRoot.Execute()
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	output := buf.String()

	// Verify it's a single line (only one newline at the end)
	if strings.Count(output, "\n") != 1 {
		t.Errorf("expected single-line JSON output, got %d newlines:\n%s", strings.Count(output, "\n"), output)
	}

	// Verify it's valid JSON
	var result map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, output)
	}

	// Verify fields
	if result["version"] != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", result["version"])
	}
	if result["commit"] != "abc123" {
		t.Errorf("expected commit 'abc123', got '%s'", result["commit"])
	}
	if result["date"] != "2024-01-01" {
		t.Errorf("expected date '2024-01-01', got '%s'", result["date"])
	}
}

func TestStatusCommandJSONFormat(t *testing.T) {
	// Save original values
	origVersion := version.Version
	origCommit := version.Commit
	origDate := version.Date

	// Set test values
	version.Version = "1.0.0"
	version.Commit = "abc123"
	version.Date = "2024-01-01"

	// Restore after test
	defer func() {
		version.Version = origVersion
		version.Commit = origCommit
		version.Date = origDate
		statusFormat = "" // Reset flag
	}()

	// Capture output
	var buf bytes.Buffer

	// Create a new root command for testing
	testRoot := &cobra.Command{Use: "costa"}
	testRoot.AddCommand(statusCmd)
	testRoot.SetOut(&buf)
	testRoot.SetErr(&buf)
	testRoot.SetArgs([]string{"status", "--format", "json"})

	err := testRoot.Execute()
	if err != nil {
		t.Fatalf("status command failed: %v", err)
	}

	output := buf.String()

	// Verify it's a single line (only one newline at the end)
	if strings.Count(output, "\n") != 1 {
		t.Errorf("expected single-line JSON output, got %d newlines:\n%s", strings.Count(output, "\n"), output)
	}

	// Verify it's valid JSON
	var result map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, output)
	}

	// Verify fields
	if result["version"] != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", result["version"])
	}
	if result["build"] != "abc123" {
		t.Errorf("expected build 'abc123', got '%s'", result["build"])
	}
	if result["date"] != "2024-01-01" {
		t.Errorf("expected date '2024-01-01', got '%s'", result["date"])
	}
}

func TestTokenCommandJSONFormatNotLoggedIn(t *testing.T) {
	// Isolate HOME so no real token exists
	t.Setenv("HOME", t.TempDir())
	// Capture output
	var buf bytes.Buffer

	// Create a new root command for testing
	testRoot := &cobra.Command{Use: "costa"}
	testRoot.AddCommand(tokenCmd)
	testRoot.SetOut(&buf)
	testRoot.SetErr(&buf)
	testRoot.SetArgs([]string{"token", "--format", "json"})

	// Reset flags after test
	defer func() {
		tokenFormat = ""
	}()

	err := testRoot.Execute()
	if err != nil {
		t.Fatalf("token command failed: %v", err)
	}

	output := buf.String()

	// Verify it's a single line (only one newline at the end)
	if strings.Count(output, "\n") != 1 {
		t.Errorf("expected single-line JSON output, got %d newlines:\n%s", strings.Count(output, "\n"), output)
	}

	// Verify it's valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, output)
	}

	// Verify logged_in is false
	loggedIn, ok := result["logged_in"].(bool)
	if !ok {
		t.Errorf("expected logged_in to be a boolean")
	}
	if loggedIn {
		t.Errorf("expected logged_in to be false, got true")
	}
}
