package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

func TestSetupStatus_CodexJSON(t *testing.T) {
	// Setup temp directory with configured codex
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".codex")
	configPath := filepath.Join(configDir, "config.toml")

	// Create directory
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Write codex config
	codexConfig := map[string]any{
		"model":          "costa/auto",
		"model_provider": "costa",
	}

	data, _ := toml.Marshal(codexConfig)
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

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

	// Run setup status codex with --format json
	root.SetArgs([]string{"setup", "status", "codex", "--format", "json"})

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
	if _, ok := result["model"]; !ok {
		t.Errorf("expected model field")
	}
}

func TestSetupStatus_HumanReadableOutput(t *testing.T) {
	// Setup temp directory
	tmpDir := t.TempDir()

	// Mock HOME to point to temp dir (so nothing is detected)
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

	// Run setup status without format flag (human-readable)
	root.SetArgs([]string{"setup", "status"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	output := buf.String()

	// Verify human-readable output includes expected sections
	if !strings.Contains(output, "Costa Setup Status") {
		t.Errorf("Expected header in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Claude Code:") {
		t.Errorf("Expected Claude Code section, got:\n%s", output)
	}
	if !strings.Contains(output, "Codex:") {
		t.Errorf("Expected Codex section, got:\n%s", output)
	}
}

func TestSetupStatus_ClaudeCode_NotConfigured(t *testing.T) {
	// Setup temp directory (no config files)
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

	// Run setup status claude-code
	root.SetArgs([]string{"setup", "status", "claude-code"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	output := buf.String()

	// Verify output indicates not configured
	if !strings.Contains(output, "Not configured") {
		t.Errorf("Expected 'Not configured' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "costa setup claude-code") {
		t.Errorf("Expected setup suggestion in output, got:\n%s", output)
	}
}

func TestSetupStatus_ClaudeCode_Configured(t *testing.T) {
	// Setup temp directory with configured Claude
	tmpDir := t.TempDir()
	settingsDir := filepath.Join(tmpDir, ".claude")
	settingsPath := filepath.Join(settingsDir, "settings.json")

	// Create directory
	if err := os.MkdirAll(settingsDir, 0700); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Write fully configured settings
	settings := map[string]any{
		"model":                 "costa/auto",
		"alwaysThinkingEnabled": true,
		"env": map[string]any{
			"ANTHROPIC_BASE_URL":               "https://ai.costa.app/api",
			"ANTHROPIC_AUTH_TOKEN":             "test-token-redacted",
			"ANTHROPIC_DEFAULT_TEXT_MODEL":     "costa/auto",
			"ANTHROPIC_DEFAULT_MESSAGES_MODEL": "costa/auto",
			"ANTHROPIC_DEFAULT_TOOL_USE_MODEL": "costa/auto",
			"CLAUDE_CODE_SUBAGENT_MODEL":       "costa/auto",
			"DISABLE_PROMPT_CACHING":           true,
		},
	}

	data, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0600); err != nil {
		t.Fatalf("Failed to write settings: %v", err)
	}

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

	// Run setup status claude-code
	root.SetArgs([]string{"setup", "status", "claude-code"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	output := buf.String()

	// Verify output indicates configured for Costa
	if !strings.Contains(output, "Configured for Costa") {
		t.Errorf("Expected 'Configured for Costa' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "costa/auto") {
		t.Errorf("Expected model 'costa/auto' in output, got:\n%s", output)
	}
}

func TestSetupStatus_ClaudeCode_PartiallyConfigured(t *testing.T) {
	// Setup temp directory with partially configured Claude
	tmpDir := t.TempDir()
	settingsDir := filepath.Join(tmpDir, ".claude")
	settingsPath := filepath.Join(settingsDir, "settings.json")

	// Create directory
	if err := os.MkdirAll(settingsDir, 0700); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Write partially configured settings (missing some required keys)
	settings := map[string]any{
		"model": "costa/auto",
		"env": map[string]any{
			"ANTHROPIC_BASE_URL":   "https://ai.costa.app/api",
			"ANTHROPIC_AUTH_TOKEN": "test-token",
			// Missing other required env vars
		},
	}

	data, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0600); err != nil {
		t.Fatalf("Failed to write settings: %v", err)
	}

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

	// Run setup status claude-code
	root.SetArgs([]string{"setup", "status", "claude-code"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	output := buf.String()

	// Verify output indicates partially configured
	if !strings.Contains(output, "Partially configured") {
		t.Errorf("Expected 'Partially configured' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Missing Costa settings:") {
		t.Errorf("Expected missing settings section in output, got:\n%s", output)
	}
	if !strings.Contains(output, "costa setup claude-code") {
		t.Errorf("Expected setup suggestion in output, got:\n%s", output)
	}
}

func TestFormatStatusIcon(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		isCosta  bool
	}{
		{
			name:     "configured for Costa",
			isCosta:  true,
			expected: "✓",
		},
		{
			name:     "not configured for Costa",
			isCosta:  false,
			expected: "⚠",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatStatusIcon(tt.isCosta)
			if result != tt.expected {
				t.Errorf("formatStatusIcon(%v) = %q, want %q", tt.isCosta, result, tt.expected)
			}
		})
	}
}

func TestSetupStatus_UnknownApp(t *testing.T) {
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

	// Run setup status with unknown app
	root.SetArgs([]string{"setup", "status", "unknown-app"})

	err := root.Execute()
	if err == nil {
		t.Fatal("Expected error for unknown app, but got none")
	}

	if !strings.Contains(err.Error(), "unknown app") {
		t.Errorf("Expected 'unknown app' error message, got: %v", err)
	}
}

func TestSetupStatus_AliasNormalization(t *testing.T) {
	tests := []struct {
		name  string
		alias string
	}{
		{"claude alias", "claude"},
		{"claude code alias", "claude code"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

			// Run setup status with alias
			root.SetArgs([]string{"setup", "status", tt.alias})

			err := root.Execute()
			if err != nil {
				t.Fatalf("Command failed: %v", err)
			}

			output := buf.String()

			// Verify it shows Claude Code status (not an error about unknown app)
			if !strings.Contains(output, "Claude Code Setup Status") {
				t.Errorf("Expected Claude Code status for alias %q, got:\n%s", tt.alias, output)
			}
		})
	}
}
