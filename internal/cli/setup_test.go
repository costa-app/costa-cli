package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestSetupClaudeCode_DeclinePrompt_DoesNotWrite(t *testing.T) {
	// Setup temp directory with existing config
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, ".claude", "settings.json")
	onboardingPath := filepath.Join(tmpDir, ".claude.json")

	// Create directory
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0700); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Write existing config with old token
	existingConfig := map[string]any{
		"model":                 "costa/auto",
		"alwaysThinkingEnabled": true,
		"statusLine":            map[string]any{"command": "costa status --format claude-code"},
		"env": map[string]any{
			"ANTHROPIC_BASE_URL":               "https://ai.costa.app/api",
			"ANTHROPIC_AUTH_TOKEN":             "old-token-must-not-change",
			"ANTHROPIC_DEFAULT_TEXT_MODEL":     "costa/auto",
			"ANTHROPIC_DEFAULT_MESSAGES_MODEL": "costa/auto",
			"ANTHROPIC_DEFAULT_TOOL_USE_MODEL": "costa/auto",
			"CLAUDE_CODE_SUBAGENT_MODEL":       "costa/auto",
			"DISABLE_PROMPT_CACHING":           true,
		},
	}

	data, _ := json.MarshalIndent(existingConfig, "", "  ")
	originalData := string(data)
	if err := os.WriteFile(settingsPath, data, 0600); err != nil {
		t.Fatalf("Failed to write existing config: %v", err)
	}

	// Write onboarding file
	onboardingConfig := map[string]any{
		"hasCompletedOnboarding": true,
	}
	onboardingData, _ := json.MarshalIndent(onboardingConfig, "", "  ")
	if err := os.WriteFile(onboardingPath, onboardingData, 0600); err != nil {
		t.Fatalf("Failed to write onboarding file: %v", err)
	}

	// Mock HOME to point to temp dir
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Prepare command with stdin set to "n\nn\n" (decline statusline, then decline proceed)
	var outBuf, errBuf bytes.Buffer
	stdinReader := strings.NewReader("n\nn\n")

	// Create root and add setup command
	root := &cobra.Command{Use: "costa"}
	root.AddCommand(setupCmd)
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetIn(stdinReader)

	// Run setup with explicit token (to avoid needing real auth)
	root.SetArgs([]string{"setup", "claude-code", "--token", "new-token-different"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Verify output contains the proceed prompt
	output := outBuf.String()
	if !strings.Contains(output, "Proceed with changes? [Y/n]:") {
		t.Errorf("Expected proceed prompt in output, got:\n%s", output)
	}

	// Verify output says "Canceled"
	if !strings.Contains(output, "Canceled") {
		t.Errorf("Expected 'Cancelled' in output after declining, got:\n%s", output)
	}

	// CRITICAL: Verify file was NOT modified
	data, err = os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings after command: %v", err)
	}

	if string(data) != originalData {
		t.Errorf("File was modified despite user declining prompt!\nOriginal:\n%s\nGot:\n%s", originalData, string(data))
	}

	// Double-check token is still old
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Failed to parse settings: %v", err)
	}

	env, ok := settings["env"].(map[string]any)
	if !ok {
		t.Fatal("Expected env object")
	}

	if token, ok := env["ANTHROPIC_AUTH_TOKEN"].(string); !ok || token != "old-token-must-not-change" {
		t.Errorf("Token was changed despite declining prompt! Got: %v", token)
	}
}

func TestSetupStatus_JSONFormat(t *testing.T) {
	// Setup temp directory
	tmpDir := t.TempDir()

	// Mock HOME to point to temp dir (so Claude CLI is not detected)
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Capture output
	var buf bytes.Buffer

	// Create root and add setup command
	root := &cobra.Command{Use: "costa"}
	root.AddCommand(setupCmd)
	root.SetOut(&buf)
	root.SetErr(&buf)

	// Run setup status with --format json
	root.SetArgs([]string{"setup", "status", "--format", "json"})

	// Reset flags after test
	defer func() {
		setupStatusFormat = ""
	}()

	err := root.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
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

	// Verify claude_code field exists
	claudeCode, ok := result["claude_code"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected claude_code field in JSON output")
	}

	// Verify required fields
	if _, ok := claudeCode["installed"]; !ok {
		t.Errorf("expected installed field in claude_code")
	}
	if _, ok := claudeCode["config_exists"]; !ok {
		t.Errorf("expected config_exists field in claude_code")
	}
	if _, ok := claudeCode["is_costa_enabled"]; !ok {
		t.Errorf("expected is_costa_enabled field in claude_code")
	}
}

func TestSetupStatusClaudeCode_JSONFormat(t *testing.T) {
	// Setup temp directory
	tmpDir := t.TempDir()

	// Mock HOME to point to temp dir
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Capture output
	var buf bytes.Buffer

	// Create root and add setup command
	root := &cobra.Command{Use: "costa"}
	root.AddCommand(setupCmd)
	root.SetOut(&buf)
	root.SetErr(&buf)

	// Run setup status claude-code with --format json
	root.SetArgs([]string{"setup", "status", "claude-code", "--format", "json"})

	// Reset flags after test
	defer func() {
		setupStatusFormat = ""
	}()

	err := root.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
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

	// Verify required fields
	if _, ok := result["installed"]; !ok {
		t.Errorf("expected installed field")
	}
	if _, ok := result["scope"]; !ok {
		t.Errorf("expected scope field")
	}
	if _, ok := result["config_path"]; !ok {
		t.Errorf("expected config_path field")
	}
	if _, ok := result["config_exists"]; !ok {
		t.Errorf("expected config_exists field")
	}
	if _, ok := result["is_costa_enabled"]; !ok {
		t.Errorf("expected is_costa_enabled field")
	}
}
