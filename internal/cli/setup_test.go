package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Amirhossein-Mirzaei23/Tharness/internal/config"
)

// setupTestEnv isolates HOME so setup writes into a temp config.
func setupTestEnv(t *testing.T) (string, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home, filepath.Join(home, ".tharness", "config.yaml")
}

func TestSetupNonInteractiveWritesConfig(t *testing.T) {
	_, cfgPath := setupTestEnv(t)

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"setup", "--provider", "openai", "--api-key", "sk-abc12345", "--browser", "firefox", "--theme", "nord"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Provider != "openai" {
		t.Errorf("Provider = %q, want openai", cfg.Provider)
	}
	auth := cfg.Providers["openai"]
	if auth.APIKey != "sk-abc12345" {
		t.Errorf("APIKey = %q, want stored", auth.APIKey)
	}
	if auth.Model != "gpt-4o-mini" {
		t.Errorf("Model = %q, want provider default model", auth.Model)
	}
	if cfg.Browser.Command != "firefox" {
		t.Errorf("Browser.Command = %q, want firefox", cfg.Browser.Command)
	}
	if cfg.Theme != "nord" {
		t.Errorf("Theme = %q, want nord", cfg.Theme)
	}
	if fi, err := os.Stat(cfgPath); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("config perms = %v, err %v; want 0600", fi.Mode().Perm(), err)
	}
	if !strings.Contains(out.String(), "openai") {
		t.Errorf("output %q should confirm the provider", out.String())
	}
}

func TestSetupCustomProviderRequiresBaseURL(t *testing.T) {
	setupTestEnv(t)
	cmd := NewRootCommand()
	cmd.SetArgs([]string{"setup", "--provider", "custom", "--api-key", "k"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--base-url") {
		t.Errorf("err = %v, want custom provider base-url requirement", err)
	}
}

func TestSetupRejectsUnknownProviderThemeBrowser(t *testing.T) {
	_, _ = setupTestEnv(t)
	cases := [][]string{
		{"setup", "--provider", "no-such"},
		{"setup", "--provider", "openai", "--theme", "no-such"},
		{"setup", "--provider", "openai", "--api-key", "k", "--browser", "no-such-browser"},
	}
	for _, args := range cases {
		cmd := NewRootCommand()
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Errorf("setup %v should fail", args)
		}
	}
}

func TestSetupWithoutAPIKeyForCloudProviderFails(t *testing.T) {
	_, _ = setupTestEnv(t)
	cmd := NewRootCommand()
	cmd.SetArgs([]string{"setup", "--provider", "anthropic"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--api-key") {
		t.Errorf("err = %v, want missing --api-key message", err)
	}
}

func TestSetupListProvidersAndThemes(t *testing.T) {
	_, _ = setupTestEnv(t)
	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"setup", "--list-providers"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list-providers: %v", err)
	}
	for _, want := range []string{"openai", "zai", "ollama", "OPENROUTER_API_KEY", "no key needed"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("list-providers output missing %q", want)
		}
	}

	out.Reset()
	cmd = NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"setup", "--list-themes"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list-themes: %v", err)
	}
	for _, want := range []string{"dark", "light", "dracula", "nord", "gruvbox"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("list-themes output missing %q", want)
		}
	}
}

func TestSetupNonInteractivePreservesOtherProviders(t *testing.T) {
	_, cfgPath := setupTestEnv(t)
	// Seed an existing zai entry, then configure openai.
	seed := "providers:\n  zai:\n    api_key: zkey-123456\nbrowser:\n  command: chromium\ntheme: dracula\n"
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"setup", "--provider", "openai", "--api-key", "sk-openai-999"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Providers["zai"].APIKey != "zkey-123456" {
		t.Errorf("zai key = %q, want preserved", cfg.Providers["zai"].APIKey)
	}
	if cfg.Provider != "openai" {
		t.Errorf("Provider = %q, want switched to openai", cfg.Provider)
	}
	if cfg.Browser.Command != "chromium" || cfg.Theme != "dracula" {
		t.Errorf("browser/theme = %q/%q, want preserved", cfg.Browser.Command, cfg.Theme)
	}
}

func TestApplySetupInputsBrowserSemantics(t *testing.T) {
	cfg := config.Config{Browser: config.Browser{Command: "chromium"}, Providers: map[string]config.ProviderAuth{}}
	applySetupInputs(&cfg, setupInputs{Provider: "openai", APIKey: "k", Browser: "auto"})
	if cfg.Browser.Command != "" {
		t.Errorf("Browser.Command = %q, want cleared by auto", cfg.Browser.Command)
	}
	applySetupInputs(&cfg, setupInputs{Provider: "openai", APIKey: "k", Browser: ""})
	if cfg.Browser.Command != "" {
		t.Errorf("empty browser input should leave config untouched, got %q", cfg.Browser.Command)
	}
	applySetupInputs(&cfg, setupInputs{Provider: "openai", APIKey: "k", Browser: "firefox"})
	if cfg.Browser.Command != "firefox" {
		t.Errorf("Browser.Command = %q, want firefox", cfg.Browser.Command)
	}
}

func TestSetupWizardRequiresTTY(t *testing.T) {
	// Under `go test` stdin is not a character device, so the wizard must
	// refuse to start instead of hanging.
	if err := runSetupWizard("", os.Stdout); err == nil || !strings.Contains(err.Error(), "terminal") {
		t.Errorf("err = %v, want interactive-terminal requirement", err)
	}
}

func TestMaskKeyHidesMiddle(t *testing.T) {
	if got := maskKey("sk-abcdef1234567890xyz"); !strings.HasPrefix(got, "sk-a") || !strings.HasSuffix(got, "0xyz") || strings.Contains(got, "cdef") {
		t.Errorf("maskKey = %q, want head/tail only", got)
	}
	if got := maskKey(""); got != "(none)" {
		t.Errorf("maskKey(\"\") = %q, want (none)", got)
	}
	if got := maskKey("short"); !strings.Contains(got, "•") {
		t.Errorf("maskKey(short) = %q, want fully masked", got)
	}
}
