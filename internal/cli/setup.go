package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/costa-app/costa-cli/internal/integrations"
	"github.com/costa-app/costa-cli/internal/integrations/claudecode"
	"github.com/costa-app/costa-cli/internal/integrations/codex"
)

var (
	setupUser             bool
	setupProject          bool
	setupToken            string
	setupForce            bool
	setupDryRun           bool
	setupBackupDir        string
	setupRefreshTokenOnly bool
	setupRequireInstalled bool
	setupStatusFormat     string
	setupEnableStatusLine bool
	setupSkipStatusLine   bool
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setup integrations with Costa",
	Long:  `Setup and configure third-party tools to work with Costa.`,
}

var setupClaudeCodeCmd = &cobra.Command{
	Use:     "claude-code",
	Aliases: []string{"claude", "claude code"},
	Short:   "Setup Claude Code to use Costa",
	Long:    `Configure Claude Code (CLI and VS Code extension) to use Costa's API and token.`,
	RunE:    runSetupClaudeCode,
}

var setupCodexCmd = &cobra.Command{
	Use:   "codex",
	Short: "Setup Codex CLI to use Costa",
	Long:  `Configure Codex CLI to use Costa's API and token.`,
	RunE:  runSetupCodex,
}

var setupStatusCmd = &cobra.Command{
	Use:   "status [app]",
	Short: "Check setup status",
	Long:  `Check if tools are installed and configured to use Costa. Run without arguments to check all apps.`,
	RunE:  runSetupStatus,
}

func init() {
	// Setup apply flags
	setupClaudeCodeCmd.Flags().BoolVar(&setupUser, "user", false, "Setup for current user (default)")
	setupClaudeCodeCmd.Flags().BoolVar(&setupProject, "project", false, "Setup for current project")
	setupClaudeCodeCmd.Flags().StringVar(&setupToken, "token", "", "Use explicit token instead of fetching from Costa")
	setupClaudeCodeCmd.Flags().BoolVar(&setupForce, "force", false, "Skip confirmation prompt (auto-yes)")
	setupClaudeCodeCmd.Flags().BoolVar(&setupDryRun, "dry-run", false, "Show what would change without writing")
	setupClaudeCodeCmd.Flags().StringVar(&setupBackupDir, "backup-dir", "", "Custom backup directory")
	setupClaudeCodeCmd.Flags().BoolVar(&setupRefreshTokenOnly, "refresh-token-only", false, "Only update the authentication token")
	setupClaudeCodeCmd.Flags().BoolVar(&setupRequireInstalled, "require-installed", false, "Fail if Claude CLI is not installed")
	setupClaudeCodeCmd.Flags().BoolVar(&setupEnableStatusLine, "enable-statusline", false, "Enable Claude Code status line")
	setupClaudeCodeCmd.Flags().BoolVar(&setupSkipStatusLine, "skip-statusline", false, "Skip statusline prompt")

	// Status flags
	setupStatusCmd.Flags().BoolVar(&setupUser, "user", false, "Check user config (default)")
	setupStatusCmd.Flags().BoolVar(&setupProject, "project", false, "Check project config")
	setupStatusCmd.Flags().StringVar(&setupStatusFormat, "format", "", "Output format (json)")

	// Codex flags
	setupCodexCmd.Flags().StringVar(&setupToken, "token", "", "Use explicit token instead of fetching from Costa")
	setupCodexCmd.Flags().BoolVar(&setupForce, "force", false, "Skip confirmation prompt (auto-yes)")
	setupCodexCmd.Flags().BoolVar(&setupDryRun, "dry-run", false, "Show what would change without writing")

	setupCmd.AddCommand(setupClaudeCodeCmd)
	setupCmd.AddCommand(setupCodexCmd)
	setupCmd.AddCommand(setupStatusCmd)
}

func runSetupClaudeCode(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Use a single reader for all prompts to avoid buffering issues
	inputReader := bufio.NewReader(cmd.InOrStdin())

	// Determine scope (default to user)
	scope := integrations.ScopeUser
	if setupProject {
		scope = integrations.ScopeProject
	}

	// Build options
	opts := integrations.ApplyOpts{
		Scope:            scope,
		TokenOverride:    setupToken,
		Force:            setupForce,
		RefreshTokenOnly: setupRefreshTokenOnly,
		DryRun:           setupDryRun,
		BackupDir:        setupBackupDir,
		RequireInstalled: setupRequireInstalled,
		EnableStatusLine: setupEnableStatusLine,
		SkipStatusLine:   setupSkipStatusLine,
	}

	// Create integration
	integration := claudecode.New()

	// Get status first to show context
	status, err := integration.Status(ctx, scope)
	if err != nil {
		return fmt.Errorf("failed to check status: %w", err)
	}

	// Show detection info
	if status.Installed {
		fmt.Fprintf(cmd.OutOrStdout(), "✓ Claude CLI detected: %s\n", status.Version)
	} else {
		if setupRequireInstalled {
			return fmt.Errorf("claude CLI not found; install it first: https://docs.claude.com/en/docs/claude-code/quickstart")
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "⚠ Claude CLI not detected (will configure anyway)\n")
	}

	fmt.Fprintf(cmd.OutOrStdout(), "📁 Config path: %s\n", status.ConfigPath)

	// Phase 1: plan (dry run) to compute changes without writing
	planOpts := opts
	planOpts.DryRun = true
	planResult, err := integration.Apply(ctx, planOpts)
	if err != nil {
		return err
	}

	// Check if already configured
	if !planResult.Changed {
		fmt.Fprintln(cmd.OutOrStdout(), "✓ Already configured! No changes needed.")
		return nil
	}

	// Show planned changes
	fmt.Fprintln(cmd.OutOrStdout(), "\n📝 Changes to apply:")
	for _, change := range planResult.UpdatedKeys {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", change)
	}

	// Honor --dry-run (show but do not write)
	if setupDryRun {
		fmt.Fprintln(cmd.OutOrStdout(), "\n🔍 Dry run - no changes made")
		return nil
	}

	// Prompt for statusLine if not already set and not skipped
	if !setupSkipStatusLine && !setupEnableStatusLine && !setupRefreshTokenOnly {
		fmt.Fprint(cmd.OutOrStdout(), "\n📊 Would you like to include the Costa status line in Claude Code?\n")
		fmt.Fprint(cmd.OutOrStdout(), "   This will show your points usage in the Claude Code status bar.\n")
		fmt.Fprint(cmd.OutOrStdout(), "   Include status line? [Y/n]: ")
		response, _ := inputReader.ReadString('\n')
		resp := strings.ToLower(strings.TrimSpace(response))
		if resp != "n" && resp != "no" { // default YES
			setupEnableStatusLine = true
			opts.EnableStatusLine = true
			// Re-plan with statusLine enabled
			planOpts.EnableStatusLine = true
			newPlanResult, err := integration.Apply(ctx, planOpts)
			if err != nil {
				return err
			}
			planResult = newPlanResult

			// Show updated changes including statusLine
			fmt.Fprintln(cmd.OutOrStdout(), "\n📝 Updated changes to apply:")
			for _, change := range planResult.UpdatedKeys {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", change)
			}
		}
	}

	// Confirm if not --force
	if !setupForce {
		fmt.Fprint(cmd.OutOrStdout(), "\nProceed with changes? [Y/n]: ")
		response, _ := inputReader.ReadString('\n')
		resp := strings.ToLower(strings.TrimSpace(response))
		if resp == "n" || resp == "no" { // default YES
			fmt.Fprintln(cmd.OutOrStdout(), "Canceled.")
			return nil
		}
	}

	// Phase 2: write (actual apply)
	writeOpts := opts
	writeOpts.DryRun = false
	result, err := integration.Apply(ctx, writeOpts)
	if err != nil {
		return err
	}

	if result.BackupPath != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "💾 Backup created: %s\n", result.BackupPath)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "✅ Successfully configured Claude Code for Costa!")
	return nil
}

func runSetupStatus(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Determine scope
	scope := integrations.ScopeUser
	if setupProject {
		scope = integrations.ScopeProject
	}

	// If specific app requested
	if len(args) > 0 {
		appName := args[0]
		// Normalize aliases
		if appName == "claude" || appName == "claude code" {
			appName = "claude-code"
		}

		if appName == "claude-code" {
			return showClaudeCodeStatus(cmd, ctx, scope)
		}
		if appName == "codex" {
			return showCodexStatus(cmd, ctx, scope)
		}

		return fmt.Errorf("unknown app: %s", appName)
	}

	// Check Claude Code
	claudeStatus, err := claudecode.New().Status(ctx, scope)
	if err != nil && setupStatusFormat != "json" {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error checking Claude Code: %v\n", err)
	}

	// Check Codex
	codexStatus, codexErr := codex.New().Status(ctx, scope)
	if codexErr != nil && setupStatusFormat != "json" {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error checking Codex: %v\n", codexErr)
	}

	// JSON output
	if setupStatusFormat == "json" {
		output := map[string]interface{}{
			"claude_code": map[string]interface{}{
				"installed":        claudeStatus.Installed,
				"version":          claudeStatus.Version,
				"config_exists":    claudeStatus.ConfigExists,
				"is_costa_enabled": claudeStatus.IsCosta,
			},
			"codex": map[string]interface{}{
				"config_exists":    codexStatus.ConfigExists,
				"is_costa_enabled": codexStatus.IsCosta,
			},
		}
		if err != nil {
			output["error"] = err.Error()
		}
		data, jsonErr := json.Marshal(output)
		if jsonErr != nil {
			return jsonErr
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	// Human-readable output
	fmt.Fprintln(cmd.OutOrStdout(), "🔍 Costa Setup Status")

	if err == nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Claude Code:    %s\n", formatStatusIcon(claudeStatus.IsCosta))
		if claudeStatus.Installed {
			fmt.Fprintf(cmd.OutOrStdout(), "  Installed:    ✓ %s\n", claudeStatus.Version)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "  Installed:    ✗ Not found")
		}
		if claudeStatus.ConfigExists {
			if claudeStatus.IsCosta {
				fmt.Fprintln(cmd.OutOrStdout(), "  Configured:   ✓ Costa enabled")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "  Configured:   ⚠ Partial setup")
			}
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "  Configured:   ✗ Not configured")
		}
	}

	if codexErr == nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Codex:          %s\n", formatStatusIcon(codexStatus.IsCosta))
		if codexStatus.ConfigExists {
			if codexStatus.IsCosta {
				fmt.Fprintln(cmd.OutOrStdout(), "  Configured:   ✓ Costa enabled")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "  Configured:   ⚠ Partial setup")
			}
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "  Configured:   ✗ Not configured")
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nRun 'costa setup status <app>' for details.\n")

	return nil
}

func showClaudeCodeStatus(cmd *cobra.Command, ctx context.Context, scope integrations.Scope) error {
	integration := claudecode.New()
	status, err := integration.Status(ctx, scope)
	if err != nil {
		return fmt.Errorf("failed to check status: %w", err)
	}

	// JSON output
	if setupStatusFormat == "json" {
		output := map[string]interface{}{
			"installed":        status.Installed,
			"version":          status.Version,
			"scope":            string(status.Scope),
			"config_path":      status.ConfigPath,
			"config_exists":    status.ConfigExists,
			"is_costa_enabled": status.IsCosta,
		}
		if status.Model != "" {
			output["model"] = status.Model
		}
		if status.TokenRedacted != "" {
			output["token_redacted"] = status.TokenRedacted
		}
		if len(status.Missing) > 0 {
			output["missing"] = status.Missing
		}
		data, jsonErr := json.Marshal(output)
		if jsonErr != nil {
			return jsonErr
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	// Human-readable output
	fmt.Fprintln(cmd.OutOrStdout(), "🔍 Claude Code Setup Status")

	// Claude CLI
	if status.Installed {
		fmt.Fprintf(cmd.OutOrStdout(), "Claude CLI:     ✓ Installed (%s)\n", status.Version)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Claude CLI:     ✗ Not found")
	}

	// Config info
	fmt.Fprintf(cmd.OutOrStdout(), "Config scope:   %s\n", status.Scope)
	fmt.Fprintf(cmd.OutOrStdout(), "Config path:    %s\n", status.ConfigPath)

	// Config status
	if !status.ConfigExists {
		fmt.Fprintln(cmd.OutOrStdout(), "Config status:  ✗ Not configured")
		fmt.Fprintln(cmd.OutOrStdout(), "Run 'costa setup claude-code' to configure.")
		return nil
	}

	if status.IsCosta {
		fmt.Fprintln(cmd.OutOrStdout(), "Config status:  ✓ Configured for Costa")

		// Show current model
		if status.Model != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Model:          %s\n", status.Model)
		}

		// Check token presence (redacted)
		if status.TokenRedacted != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Token:          %s\n", status.TokenRedacted)
		}
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Config status:  ⚠ Partially configured")
		if len(status.Missing) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "\nMissing Costa settings:")
			for _, key := range status.Missing {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", key)
			}
		}
		fmt.Fprintln(cmd.OutOrStdout(), "\nRun 'costa setup claude-code' to fix.")
	}

	return nil
}

func showCodexStatus(cmd *cobra.Command, ctx context.Context, scope integrations.Scope) error {
	integration := codex.New()
	status, err := integration.Status(ctx, scope)
	if err != nil {
		return fmt.Errorf("failed to check status: %w", err)
	}

	// JSON output
	if setupStatusFormat == "json" {
		output := map[string]interface{}{
			"scope":            string(status.Scope),
			"config_path":      status.ConfigPath,
			"config_exists":    status.ConfigExists,
			"is_costa_enabled": status.IsCosta,
		}
		if status.Model != "" {
			output["model"] = status.Model
		}
		data, jsonErr := json.Marshal(output)
		if jsonErr != nil {
			return jsonErr
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	// Human-readable output
	fmt.Fprintln(cmd.OutOrStdout(), "🔍 Codex Setup Status")

	// Config info
	fmt.Fprintf(cmd.OutOrStdout(), "Config scope:   %s\n", status.Scope)
	fmt.Fprintf(cmd.OutOrStdout(), "Config path:    %s\n", status.ConfigPath)

	// Config status
	if !status.ConfigExists {
		fmt.Fprintln(cmd.OutOrStdout(), "Config status:  ✗ Not configured")
		fmt.Fprintln(cmd.OutOrStdout(), "Run 'costa setup codex' to configure.")
		return nil
	}

	if status.IsCosta {
		fmt.Fprintln(cmd.OutOrStdout(), "Config status:  ✓ Configured for Costa")

		// Show current model
		if status.Model != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Model:          %s\n", status.Model)
		}
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Config status:  ⚠ Partially configured")
		fmt.Fprintln(cmd.OutOrStdout(), "\nRun 'costa setup codex' to fix.")
	}

	return nil
}

func runSetupCodex(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	opts := integrations.ApplyOpts{
		Scope:         integrations.ScopeUser,
		TokenOverride: setupToken,
		Force:         setupForce,
		DryRun:        setupDryRun,
	}

	integration := codex.New()

	// Get status
	status, err := integration.Status(ctx, integrations.ScopeUser)
	if err != nil {
		return fmt.Errorf("failed to check status: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "📁 Config path: %s\n", status.ConfigPath)

	// Phase 1: dry run to see changes
	planOpts := opts
	planOpts.DryRun = true
	planResult, err := integration.Apply(ctx, planOpts)
	if err != nil {
		return err
	}

	if !planResult.Changed {
		fmt.Fprintln(cmd.OutOrStdout(), "✓ Already configured! No changes needed.")
		return nil
	}

	// Show planned changes
	fmt.Fprintln(cmd.OutOrStdout(), "\n📝 Changes to apply:")
	for _, change := range planResult.UpdatedKeys {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", change)
	}

	// Honor --dry-run
	if setupDryRun {
		fmt.Fprintln(cmd.OutOrStdout(), "\n🔍 Dry run - no changes made")
		return nil
	}

	// Confirm if not --force
	if !setupForce {
		fmt.Fprint(cmd.OutOrStdout(), "\nProceed with changes? [Y/n]: ")
		var response string
		fmt.Scanln(&response)
		resp := strings.ToLower(strings.TrimSpace(response))
		if resp == "n" || resp == "no" {
			fmt.Fprintln(cmd.OutOrStdout(), "Canceled.")
			return nil
		}
	}

	// Phase 2: apply
	writeOpts := opts
	writeOpts.DryRun = false
	_, err = integration.Apply(ctx, writeOpts)
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), "✅ Successfully configured Codex for Costa!")
	return nil
}

func formatStatusIcon(isCosta bool) string {
	if isCosta {
		return "✓"
	}
	return "⚠"
}
