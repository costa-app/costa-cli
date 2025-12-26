package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/costa-app/costa-cli/internal/integrations"
	"github.com/costa-app/costa-cli/internal/integrations/claudecode"
)

var (
	ccSetupUser             bool
	ccSetupProject          bool
	ccSetupToken            string
	ccSetupForce            bool
	ccSetupDryRun           bool
	ccSetupBackupDir        string
	ccSetupRefreshTokenOnly bool
	ccSetupRequireInstalled bool
	ccSetupEnableStatusLine bool
	ccSetupSkipStatusLine   bool
)

var setupClaudeCodeCmd = &cobra.Command{
	Use:     "claude-code",
	Aliases: []string{"claude", "claude code"},
	Short:   "Setup Claude Code to use Costa",
	Long:    `Configure Claude Code (CLI and VS Code extension) to use Costa's API and token.`,
	RunE:    runSetupClaudeCode,
}

func init() {
	setupClaudeCodeCmd.Flags().BoolVar(&ccSetupUser, "user", false, "Setup for current user (default)")
	setupClaudeCodeCmd.Flags().BoolVar(&ccSetupProject, "project", false, "Setup for current project")
	setupClaudeCodeCmd.Flags().StringVar(&ccSetupToken, "token", "", "Use explicit token instead of fetching from Costa")
	setupClaudeCodeCmd.Flags().BoolVar(&ccSetupForce, "force", false, "Skip confirmation prompt (auto-yes)")
	setupClaudeCodeCmd.Flags().BoolVar(&ccSetupDryRun, "dry-run", false, "Show what would change without writing")
	setupClaudeCodeCmd.Flags().StringVar(&ccSetupBackupDir, "backup-dir", "", "Custom backup directory")
	setupClaudeCodeCmd.Flags().BoolVar(&ccSetupRefreshTokenOnly, "refresh-token-only", false, "Only update the authentication token")
	setupClaudeCodeCmd.Flags().BoolVar(&ccSetupRequireInstalled, "require-installed", false, "Fail if Claude CLI is not installed")
	setupClaudeCodeCmd.Flags().BoolVar(&ccSetupEnableStatusLine, "enable-statusline", false, "Enable Claude Code status line")
	setupClaudeCodeCmd.Flags().BoolVar(&ccSetupSkipStatusLine, "skip-statusline", false, "Skip statusline prompt")
}

func runSetupClaudeCode(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Use a single reader for all prompts to avoid buffering issues
	inputReader := bufio.NewReader(cmd.InOrStdin())

	// Determine scope (default to user)
	scope := integrations.ScopeUser
	if ccSetupProject {
		scope = integrations.ScopeProject
	}

	// Build options
	opts := integrations.ApplyOpts{
		Scope:            scope,
		TokenOverride:    ccSetupToken,
		Force:            ccSetupForce,
		RefreshTokenOnly: ccSetupRefreshTokenOnly,
		DryRun:           ccSetupDryRun,
		BackupDir:        ccSetupBackupDir,
		RequireInstalled: ccSetupRequireInstalled,
		EnableStatusLine: ccSetupEnableStatusLine,
		SkipStatusLine:   ccSetupSkipStatusLine,
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
		if ccSetupRequireInstalled {
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
	if ccSetupDryRun {
		fmt.Fprintln(cmd.OutOrStdout(), "\n🔍 Dry run - no changes made")
		return nil
	}

	// Prompt for statusLine if not already set and not skipped
	if !ccSetupSkipStatusLine && !ccSetupEnableStatusLine && !ccSetupRefreshTokenOnly {
		fmt.Fprint(cmd.OutOrStdout(), "\n📊 Would you like to include the Costa status line in Claude Code?\n")
		fmt.Fprint(cmd.OutOrStdout(), "   This will show your points usage in the Claude Code status bar.\n")
		fmt.Fprint(cmd.OutOrStdout(), "   Include status line? [Y/n]: ")
		response, _ := inputReader.ReadString('\n')
		resp := strings.ToLower(strings.TrimSpace(response))
		if resp != "n" && resp != "no" { // default YES
			ccSetupEnableStatusLine = true
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
	if !ccSetupForce {
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
