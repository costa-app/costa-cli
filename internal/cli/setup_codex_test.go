package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

func TestSetupCodex_DryRun(t *testing.T) {
	// Setup temp directory
	tmpDir := t.TempDir()

	// Mock HOME to point to temp dir
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Capture output
	var outBuf, errBuf bytes.Buffer

	// Create root and add setup command
	root := &cobra.Command{Use: "costa"}
	root.AddCommand(setupCmd)
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)

	// Run setup with dry-run flag
	root.SetArgs([]string{"setup", "codex", "--token", "test-token", "--dry-run"})

	// Reset flags after test
	defer func() {
		cdSetupDryRun = false
		cdSetupToken = ""
	}()

	err := root.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	output := outBuf.String()

	// Verify dry-run message is present
	if !strings.Contains(output, "Dry run - no changes made") {
		t.Errorf("Expected dry-run message in output, got:\n%s", output)
	}

	// Verify changes are shown
	if !strings.Contains(output, "Changes to apply:") {
		t.Errorf("Expected changes list in output, got:\n%s", output)
	}

	// Verify no config file was created
	configPath := filepath.Join(tmpDir, ".codex", "config.toml")
	if _, err := os.Stat(configPath); err == nil {
		t.Errorf("Expected no config file to be created in dry-run mode, but found: %s", configPath)
	}
}

func TestSetupCodex_ForceSkipsPrompts(t *testing.T) {
	// Setup temp directory
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".codex")
	configPath := filepath.Join(configDir, "config.toml")

	// Create directory
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Mock HOME to point to temp dir
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Capture output
	var outBuf, errBuf bytes.Buffer

	// Create root and add setup command
	root := &cobra.Command{Use: "costa"}
	root.AddCommand(setupCmd)
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)

	// Run setup with force flag (should skip prompts)
	root.SetArgs([]string{"setup", "codex", "--token", "test-token", "--force"})

	// Reset flags after test
	defer func() {
		cdSetupForce = false
		cdSetupToken = ""
	}()

	err := root.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	output := outBuf.String()

	// Verify no prompts are shown
	if strings.Contains(output, "Proceed with changes?") {
		t.Errorf("Expected no proceed prompt with --force, got:\n%s", output)
	}

	// Verify success message
	if !strings.Contains(output, "Successfully configured Codex for Costa") {
		t.Errorf("Expected success message in output, got:\n%s", output)
	}

	// Verify config file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("Expected config file to be created at: %s", configPath)
	}

	// Verify config contents
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var config map[string]any
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatalf("Failed to parse TOML: %v", err)
	}

	if model, ok := config["model"].(string); !ok || model != "costa/auto" {
		t.Errorf("Expected model to be 'costa/auto', got: %v", config["model"])
	}

	if provider, ok := config["model_provider"].(string); !ok || provider != "costa" {
		t.Errorf("Expected model_provider to be 'costa', got: %v", config["model_provider"])
	}
}

func TestSetupCodex_DeclinePrompt_DoesNotWrite(t *testing.T) {
	// Setup temp directory with existing config
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".codex")
	configPath := filepath.Join(configDir, "config.toml")

	// Create directory
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Write existing config with old token
	existingConfig := map[string]any{
		"model":          "costa/auto",
		"model_provider": "costa",
		"model_providers": map[string]any{
			"costa": map[string]any{
				"name":                      "costa",
				"base_url":                  "https://ai.costa.app/api/v1",
				"experimental_bearer_token": "old-token-must-not-change",
			},
		},
		"features": map[string]any{
			"web_search_request": true,
		},
	}

	data, err := toml.Marshal(existingConfig)
	if err != nil {
		t.Fatalf("Failed to marshal TOML: %v", err)
	}
	originalData := string(data)

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("Failed to write existing config: %v", err)
	}

	// Mock HOME to point to temp dir
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Prepare command with stdin set to "n\n" (decline proceed)
	var outBuf, errBuf bytes.Buffer
	stdinReader := strings.NewReader("n\n")

	// Create root and add setup command
	root := &cobra.Command{Use: "costa"}
	root.AddCommand(setupCmd)
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetIn(stdinReader)

	// Run setup with explicit token (to avoid needing real auth)
	root.SetArgs([]string{"setup", "codex", "--token", "new-token-different"})

	// Reset flags after test
	defer func() {
		cdSetupToken = ""
	}()

	err = root.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	output := outBuf.String()

	// Verify output contains the proceed prompt
	if !strings.Contains(output, "Proceed with changes? [Y/n]:") {
		t.Errorf("Expected proceed prompt in output, got:\n%s", output)
	}

	// Verify output says "Canceled"
	if !strings.Contains(output, "Canceled") {
		t.Errorf("Expected 'Canceled' in output after declining, got:\n%s", output)
	}

	// CRITICAL: Verify file was NOT modified
	data, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config after command: %v", err)
	}

	if string(data) != originalData {
		t.Errorf("File was modified despite user declining prompt!\nOriginal:\n%s\nGot:\n%s", originalData, string(data))
	}

	// Double-check token is still old
	var config map[string]any
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatalf("Failed to parse TOML: %v", err)
	}

	providers, ok := config["model_providers"].(map[string]any)
	if !ok {
		t.Fatal("Expected model_providers object")
	}

	costa, ok := providers["costa"].(map[string]any)
	if !ok {
		t.Fatal("Expected costa provider object")
	}

	if token, ok := costa["experimental_bearer_token"].(string); !ok || token != "old-token-must-not-change" {
		t.Errorf("Token was changed despite declining prompt! Got: %v", costa["experimental_bearer_token"])
	}
}

func TestSetupCodex_AlreadyConfigured(t *testing.T) {
	// Setup temp directory with fully configured settings
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".codex")
	configPath := filepath.Join(configDir, "config.toml")

	// Create directory
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Write fully configured config
	existingConfig := map[string]any{
		"model":          "costa/auto",
		"model_provider": "costa",
		"model_providers": map[string]any{
			"costa": map[string]any{
				"name":                      "costa",
				"base_url":                  "https://ai.costa.app/api/v1",
				"experimental_bearer_token": "test-token",
			},
		},
		"features": map[string]any{
			"web_search_request": true,
		},
	}

	data, err := toml.Marshal(existingConfig)
	if err != nil {
		t.Fatalf("Failed to marshal TOML: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("Failed to write existing config: %v", err)
	}

	// Mock HOME to point to temp dir
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Capture output
	var outBuf, errBuf bytes.Buffer

	// Create root and add setup command
	root := &cobra.Command{Use: "costa"}
	root.AddCommand(setupCmd)
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)

	// Run setup with same token
	root.SetArgs([]string{"setup", "codex", "--token", "test-token"})

	// Reset flags after test
	defer func() {
		cdSetupToken = ""
	}()

	err = root.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	output := outBuf.String()

	// Verify "already configured" message
	if !strings.Contains(output, "Already configured! No changes needed.") {
		t.Errorf("Expected already configured message, got:\n%s", output)
	}

	// Verify no prompts shown
	if strings.Contains(output, "Proceed with changes?") {
		t.Errorf("Should not show proceed prompt when already configured, got:\n%s", output)
	}
}
