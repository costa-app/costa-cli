package cli

import (
	"testing"
)

func TestSetupRoot_RegistersSubcommands(t *testing.T) {
	// setupCmd is defined in setup_root.go and should have subcommands wired in init()
	names := map[string]bool{}
	for _, c := range setupCmd.Commands() {
		names[c.Use] = true
	}
	if !names["claude-code"] {
		t.Errorf("expected claude-code subcommand to be registered")
	}
	if !names["codex"] {
		t.Errorf("expected codex subcommand to be registered")
	}
	if !names["status [app]"] {
		t.Errorf("expected status subcommand to be registered with Use 'status [app]'")
	}
}
