package codex

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/costa-app/costa-cli/internal/auth"
	"github.com/costa-app/costa-cli/internal/debug"
	"github.com/costa-app/costa-cli/internal/integrations"
)

// Codex implements the Integration interface for Codex CLI
// It manages ~/.codex/config.toml and env export in shell profile

type Codex struct{}

func New() *Codex { return &Codex{} }

func (c *Codex) Name() string { return "codex" }

// Apply configures Codex per user scope only (project scope not supported)
func (c *Codex) Apply(ctx context.Context, opts integrations.ApplyOpts) (integrations.ApplyResult, error) {
	res := integrations.ApplyResult{}

	// Resolve config path
	cfgPath, err := resolveConfigPath()
	if err != nil {
		return res, err
	}
	res.ConfigPath = cfgPath

	// Load existing TOML if present
	existing := map[string]any{}
	if data, err := os.ReadFile(cfgPath); err == nil {
		if err := toml.Unmarshal(data, &existing); err != nil {
			return res, fmt.Errorf("failed parsing %s: %w", cfgPath, err)
		}
	}

	// Verify token is available (required for shell profile setup in CLI)
	if opts.TokenOverride == "" {
		debug.Printf("Fetching coding token from Costa...\n")
		_, err := auth.GetCodingToken(ctx)
		if err != nil {
			return res, fmt.Errorf("failed to get Costa token: %w\nRun 'costa login' first", err)
		}
	}

	// Build desired structure
	desired := map[string]any{
		"model_provider": "costa",
		"model":          "costa/auto",
		"features": map[string]any{
			"web_search_request": true,
		},
		"model_providers": map[string]any{
			"costa": map[string]any{
				"name":     "costa",
				"base_url": auth.GetBaseURL() + "/api/v1",
				"env_key":  "COSTA_KEY",
			},
		},
	}

	// Merge desired into existing
	updated, updatedKeys := mergeToml(existing, desired)
	res.UpdatedKeys = updatedKeys
	res.Changed = len(updatedKeys) > 0

	if opts.DryRun || !res.Changed {
		return res, nil
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0700); err != nil {
		return res, err
	}

	// Write TOML atomically
	bytes, err := toml.Marshal(updated)
	if err != nil {
		return res, err
	}
	tmp := cfgPath + ".tmp"
	if err := os.WriteFile(tmp, bytes, 0600); err != nil {
		return res, err
	}
	if err := os.Rename(tmp, cfgPath); err != nil {
		return res, err
	}

	return res, nil
}

// Status reports Codex status
func (c *Codex) Status(ctx context.Context, scope integrations.Scope) (integrations.StatusResult, error) {
	res := integrations.StatusResult{Scope: integrations.ScopeUser}

	cfgPath, err := resolveConfigPath()
	if err != nil {
		return res, err
	}
	res.ConfigPath = cfgPath

	if data, err := os.ReadFile(cfgPath); err == nil {
		res.ConfigExists = true
		var m map[string]any
		if err := toml.Unmarshal(data, &m); err == nil {
			if mp, ok := m["model"].(string); ok {
				res.Model = mp
			}
			// determine if costa configured
			if prov, ok := m["model_provider"].(string); ok && prov == "costa" {
				res.IsCosta = true
			}
		}
	}
	return res, nil
}

func resolveConfigPath() (string, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".codex", "config.toml"), nil
}

// mergeToml does a shallow merge and tracks updated keys
func mergeToml(existing, desired map[string]any) (map[string]any, []string) {
	updated := map[string]any{}
	for k, v := range existing {
		updated[k] = v
	}

	var updatedKeys []string
	apply := func(path string) {
		updatedKeys = append(updatedKeys, path)
	}

	// top-level
	if existing["model_provider"] != desired["model_provider"] {
		updated["model_provider"] = desired["model_provider"]
		apply("model_provider")
	}
	if existing["model"] != desired["model"] {
		updated["model"] = desired["model"]
		apply("model")
	}

	// features
	feat := map[string]any{}
	if v, ok := existing["features"].(map[string]any); ok {
		feat = v
	}
	if feat["web_search_request"] != true {
		feat["web_search_request"] = true
		updated["features"] = feat
		apply("features.web_search_request")
	}

	// providers.costa
	providers := map[string]any{}
	if v, ok := existing["model_providers"].(map[string]any); ok {
		providers = v
	}
	costa := map[string]any{}
	if v, ok := providers["costa"].(map[string]any); ok {
		costa = v
	}

	if costa["name"] != "costa" {
		costa["name"] = "costa"
		apply("model_providers.costa.name")
	}
	base := auth.GetBaseURL() + "/api/v1"
	if costa["base_url"] != base {
		costa["base_url"] = base
		apply("model_providers.costa.base_url")
	}
	if costa["env_key"] != "COSTA_KEY" {
		costa["env_key"] = "COSTA_KEY"
		apply("model_providers.costa.env_key")
	}
	providers["costa"] = costa
	updated["model_providers"] = providers

	return updated, updatedKeys
}

// AddCostaKeyToShellProfile ensures COSTA_KEY is exported in the user's shell profile
func AddCostaKeyToShellProfile(token string) (string, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Detect shell from $SHELL environment variable
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		return "", fmt.Errorf("SHELL environment variable not set; cannot determine shell profile")
	}

	// Extract shell name from path (e.g., /bin/zsh -> zsh)
	shellName := filepath.Base(shellPath)

	// Determine profile file based on shell
	var profile string
	switch shellName {
	case "zsh":
		profile = filepath.Join(h, ".zprofile")
	case "bash":
		// Prefer .bash_profile on macOS, .bashrc on Linux
		if runtime.GOOS == "darwin" {
			profile = filepath.Join(h, ".bash_profile")
		} else {
			profile = filepath.Join(h, ".bashrc")
		}
	default:
		return "", fmt.Errorf("unsupported shell: %s (only bash and zsh are supported)", shellName)
	}

	line := fmt.Sprintf("export COSTA_KEY=%q\n", token)

	// Idempotent append (simple): read and check substring
	data, _ := os.ReadFile(profile)
	if !containsLine(string(data), line) {
		f, err := os.OpenFile(profile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return "", err
		}
		defer func() {
			if cerr := f.Close(); cerr != nil && err == nil {
				err = cerr
			}
		}()
		if _, err := f.WriteString(line); err != nil {
			return "", err
		}
	}
	return profile, nil
}

func containsLine(s, line string) bool {
	return strings.Contains(s, line)
}
