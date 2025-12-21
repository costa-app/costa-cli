package claudecode

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/costa-app/costa-cli/internal/integrations"
)

func TestClaudeCodeSetup_CreatesConfigWithCostaSettings(t *testing.T) {
	// Setup temp directory
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, ".claude", "settings.json")

	// Mock home directory to use temp dir
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Create integration
	integration := New()

	// Apply configuration
	opts := integrations.ApplyOpts{
		Scope:         integrations.ScopeUser,
		TokenOverride: "test-token-12345",
		Force:         true, // Auto-accept
	}

	result, err := integration.Apply(context.Background(), opts)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		t.Fatalf("Settings file was not created at %s", settingsPath)
	}

	// Load and verify contents
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings file: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Failed to parse settings JSON: %v", err)
	}

	// Check top-level keys
	if model, ok := settings["model"].(string); !ok || model != "costa/auto" {
		t.Errorf("Expected model=costa/auto, got %v", settings["model"])
	}

	if thinking, ok := settings["alwaysThinkingEnabled"].(bool); !ok || !thinking {
		t.Errorf("Expected alwaysThinkingEnabled=true, got %v", settings["alwaysThinkingEnabled"])
	}

	if apiKeyHelper, ok := settings["apiKeyHelper"].(string); !ok || apiKeyHelper != "echo $ANTHROPIC_API_KEY" {
		t.Errorf("Expected apiKeyHelper='echo $ANTHROPIC_API_KEY', got %v", settings["apiKeyHelper"])
	}

	// Check env keys
	env, ok := settings["env"].(map[string]any)
	if !ok {
		t.Fatalf("Expected env to be an object, got %T", settings["env"])
	}

	expectedEnvKeys := map[string]string{
		"ANTHROPIC_AUTH_TOKEN":             "test-token-12345",
		"ANTHROPIC_DEFAULT_TEXT_MODEL":     "costa/auto",
		"ANTHROPIC_DEFAULT_MESSAGES_MODEL": "costa/auto",
		"ANTHROPIC_DEFAULT_TOOL_USE_MODEL": "costa/auto",
		"CLAUDE_CODE_SUBAGENT_MODEL":       "costa/auto",
	}

	for key, expectedValue := range expectedEnvKeys {
		if val, ok := env[key].(string); !ok || val != expectedValue {
			t.Errorf("Expected env.%s=%s, got %v", key, expectedValue, env[key])
		}
	}

	// Check DISABLE_PROMPT_CACHING is true (bool)
	if caching, ok := env["DISABLE_PROMPT_CACHING"].(bool); !ok || !caching {
		t.Errorf("Expected env.DISABLE_PROMPT_CACHING=true (bool), got %v", env["DISABLE_PROMPT_CACHING"])
	}

	// Verify result metadata
	if !result.Changed {
		t.Error("Expected result.Changed=true for new config")
	}

	if len(result.UpdatedKeys) == 0 {
		t.Error("Expected UpdatedKeys to be populated")
	}
}

func TestClaudeCodeSetup_UpdatesTokenWhenDifferent(t *testing.T) {
	// Setup temp directory with existing config
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, ".claude", "settings.json")

	// Create directory
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0700); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Write existing config with old token
	existingConfig := map[string]any{
		"model":                 "costa/auto",
		"alwaysThinkingEnabled": true,
		"env": map[string]any{
			"ANTHROPIC_BASE_URL":               "https://ai.costa.app/api",
			"ANTHROPIC_AUTH_TOKEN":             "old-token-67890",
			"ANTHROPIC_DEFAULT_TEXT_MODEL":     "costa/auto",
			"ANTHROPIC_DEFAULT_MESSAGES_MODEL": "costa/auto",
			"ANTHROPIC_DEFAULT_TOOL_USE_MODEL": "costa/auto",
			"CLAUDE_CODE_SUBAGENT_MODEL":       "costa/auto",
			"DISABLE_PROMPT_CACHING":           true,
		},
	}

	data, _ := json.MarshalIndent(existingConfig, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0600); err != nil {
		t.Fatalf("Failed to write existing config: %v", err)
	}

	// Mock home directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Create integration
	integration := New()

	// Apply with new token (without --yes to test that changes are detected)
	opts := integrations.ApplyOpts{
		Scope:         integrations.ScopeUser,
		TokenOverride: "new-token-12345",
		DryRun:        true, // Dry run to check detection without writing
	}

	result, err := integration.Apply(context.Background(), opts)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Verify that token change was detected
	if !result.Changed {
		t.Error("Expected result.Changed=true when token differs")
	}

	// Verify that ANTHROPIC_AUTH_TOKEN is in UpdatedKeys
	foundTokenUpdate := false
	for _, key := range result.UpdatedKeys {
		if key == "env.ANTHROPIC_AUTH_TOKEN" {
			foundTokenUpdate = true
			break
		}
	}

	if !foundTokenUpdate {
		t.Errorf("Expected env.ANTHROPIC_AUTH_TOKEN in UpdatedKeys, got: %v", result.UpdatedKeys)
	}

	// Now apply for real with --yes
	opts.DryRun = false
	opts.Force = true

	result, err = integration.Apply(context.Background(), opts)
	if err != nil {
		t.Fatalf("Apply (write) failed: %v", err)
	}

	// Verify backup was created
	if result.BackupPath == "" {
		t.Error("Expected backup to be created")
	}

	// Verify token was actually updated in file
	data, err = os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read updated settings: %v", err)
	}

	var updated map[string]any
	if err := json.Unmarshal(data, &updated); err != nil {
		t.Fatalf("Failed to parse updated settings: %v", err)
	}

	env, ok := updated["env"].(map[string]any)
	if !ok {
		t.Fatalf("Expected env to be an object")
	}

	if token, ok := env["ANTHROPIC_AUTH_TOKEN"].(string); !ok || token != "new-token-12345" {
		t.Errorf("Expected token to be updated to 'new-token-12345', got %v", env["ANTHROPIC_AUTH_TOKEN"])
	}
}

func TestClaudeCodeSetup_PreservesUnknownKeys(t *testing.T) {
	// Setup temp directory with existing config that has custom keys
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, ".claude", "settings.json")

	// Create directory
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0700); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Write existing config with custom keys
	existingConfig := map[string]any{
		"model":                 "gpt-4",
		"alwaysThinkingEnabled": false,
		"customKey":             "customValue",
		"env": map[string]any{
			"CUSTOM_ENV_VAR":       "should-be-preserved",
			"ANTHROPIC_AUTH_TOKEN": "old-token",
		},
	}

	data, _ := json.MarshalIndent(existingConfig, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0600); err != nil {
		t.Fatalf("Failed to write existing config: %v", err)
	}

	// Mock home directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Create integration
	integration := New()

	// Apply (should set missing, update all Costa settings including model, preserve custom keys)
	opts := integrations.ApplyOpts{
		Scope:         integrations.ScopeUser,
		TokenOverride: "new-token-12345",
		Force:         true,
	}

	_, err := integration.Apply(context.Background(), opts)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Load and verify
	data, err = os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read updated settings: %v", err)
	}

	var updated map[string]any
	if err := json.Unmarshal(data, &updated); err != nil {
		t.Fatalf("Failed to parse updated settings: %v", err)
	}

	// Verify custom keys are preserved
	if custom, ok := updated["customKey"].(string); !ok || custom != "customValue" {
		t.Errorf("Expected customKey to be preserved, got %v", updated["customKey"])
	}

	// Verify model WAS updated (always updates when different)
	if model, ok := updated["model"].(string); !ok || model != "costa/auto" {
		t.Errorf("Expected model to be updated to costa/auto, got %v", updated["model"])
	}

	// Verify token WAS updated (always updates when different)
	env, ok := updated["env"].(map[string]any)
	if !ok {
		t.Fatalf("Expected env to be an object")
	}

	if token, ok := env["ANTHROPIC_AUTH_TOKEN"].(string); !ok || token != "new-token-12345" {
		t.Errorf("Expected token to be updated to new-token-12345, got %v", env["ANTHROPIC_AUTH_TOKEN"])
	}

	// Verify custom env var is preserved
	if custom, ok := env["CUSTOM_ENV_VAR"].(string); !ok || custom != "should-be-preserved" {
		t.Errorf("Expected CUSTOM_ENV_VAR to be preserved, got %v", env["CUSTOM_ENV_VAR"])
	}

	// Verify Costa env keys were added
	if _, ok := env["CLAUDE_CODE_SUBAGENT_MODEL"]; !ok {
		t.Error("Expected CLAUDE_CODE_SUBAGENT_MODEL to be added")
	}
}

func TestClaudeCodeSetup_UpdateFlagOverwritesExisting(t *testing.T) {
	// Setup temp directory with existing config
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, ".claude", "settings.json")

	// Create directory
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0700); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Write existing config with different model
	existingConfig := map[string]any{
		"model": "gpt-4",
		"env": map[string]any{
			"ANTHROPIC_AUTH_TOKEN": "old-token",
		},
	}

	data, _ := json.MarshalIndent(existingConfig, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0600); err != nil {
		t.Fatalf("Failed to write existing config: %v", err)
	}

	// Mock home directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Create integration
	integration := New()

	// Apply (always update differing values)
	opts := integrations.ApplyOpts{
		Scope:         integrations.ScopeUser,
		TokenOverride: "new-token-12345",
		Force:         true,
	}

	_, err := integration.Apply(context.Background(), opts)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Load and verify
	data, err = os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read updated settings: %v", err)
	}

	var updated map[string]any
	if err := json.Unmarshal(data, &updated); err != nil {
		t.Fatalf("Failed to parse updated settings: %v", err)
	}

	// Verify model WAS updated
	if model, ok := updated["model"].(string); !ok || model != "costa/auto" {
		t.Errorf("Expected model to be updated to costa/auto, got %v", updated["model"])
	}
}

func TestClaudeCodeSetup_DryRunDoesNotWrite(t *testing.T) {
	// Setup temp directory with existing config
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, ".claude", "settings.json")

	// Create directory
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0700); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Write existing config with old token
	existingConfig := map[string]any{
		"model": "costa/auto",
		"env": map[string]any{
			"ANTHROPIC_AUTH_TOKEN": "old-token-should-not-change",
		},
	}

	data, _ := json.MarshalIndent(existingConfig, "", "  ")
	originalData := string(data)
	if err := os.WriteFile(settingsPath, data, 0600); err != nil {
		t.Fatalf("Failed to write existing config: %v", err)
	}

	// Mock home directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Create integration
	integration := New()

	// Apply with DryRun=true
	opts := integrations.ApplyOpts{
		Scope:         integrations.ScopeUser,
		TokenOverride: "new-token-12345",
		DryRun:        true, // Should NOT write
	}

	result, err := integration.Apply(context.Background(), opts)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Verify changes were detected
	if !result.Changed {
		t.Error("Expected result.Changed=true when token differs")
	}

	// Verify backup was NOT created (dry run)
	if result.BackupPath != "" {
		t.Errorf("Expected no backup in dry run, got: %s", result.BackupPath)
	}

	// Verify file was NOT modified
	data, err = os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings after dry run: %v", err)
	}

	if string(data) != originalData {
		t.Error("File was modified during dry run (should not have been)")
	}

	// Verify token is still the old one
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Failed to parse settings: %v", err)
	}

	env, ok := settings["env"].(map[string]any)
	if !ok {
		t.Fatal("Expected env object")
	}

	if token, ok := env["ANTHROPIC_AUTH_TOKEN"].(string); !ok || token != "old-token-should-not-change" {
		t.Errorf("Token was changed during dry run: got %v", token)
	}
}
