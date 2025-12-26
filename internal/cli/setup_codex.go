package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/costa-app/costa-cli/internal/integrations"
	"github.com/costa-app/costa-cli/internal/integrations/codex"
)

var (
	cdSetupToken  string
	cdSetupForce  bool
	cdSetupDryRun bool
)

var setupCodexCmd = &cobra.Command{
	Use:   "codex",
	Short: "Setup Codex CLI to use Costa",
	Long:  `Configure Codex CLI to use Costa's API and token.`,
	RunE:  runSetupCodex,
}

func init() {
	setupCodexCmd.Flags().StringVar(&cdSetupToken, "token", "", "Use explicit token instead of fetching from Costa")
	setupCodexCmd.Flags().BoolVar(&cdSetupForce, "force", false, "Skip confirmation prompt (auto-yes)")
	setupCodexCmd.Flags().BoolVar(&cdSetupDryRun, "dry-run", false, "Show what would change without writing")
}

func runSetupCodex(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Use a single reader for all prompts to avoid buffering issues
	inputReader := bufio.NewReader(cmd.InOrStdin())

	opts := integrations.ApplyOpts{
		Scope:         integrations.ScopeUser,
		TokenOverride: cdSetupToken,
		Force:         cdSetupForce,
		DryRun:        cdSetupDryRun,
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
	if cdSetupDryRun {
		fmt.Fprintln(cmd.OutOrStdout(), "\n🔍 Dry run - no changes made")
		return nil
	}

	// Confirm if not --force
	if !cdSetupForce {
		fmt.Fprint(cmd.OutOrStdout(), "\nProceed with changes? [Y/n]: ")
		response, _ := inputReader.ReadString('\n')
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
