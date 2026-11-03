package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTripKeepsProviderEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	in := Config{
		Provider: "openai",
		Theme:    "nord",
		Browser:  Browser{Command: "/usr/bin/firefox"},
		Providers: map[string]ProviderAuth{
			"openai": {APIKey: "sk-test-123", BaseURL: "https://api.openai.com/v1", Model: "gpt-4o-mini"},
			"zai":    {APIKey: "zkey"},
		},
		Permissions: Permissions{Bash: "auto", Write: "ask", Edit: "auto", Browser: "deny"},
	}
	if err := Save(in, path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("config perms = %v, want 0600 (keys on disk)", fi.Mode().Perm())
	}

	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if out.Provider != "openai" || out.Theme != "nord" {
		t.Errorf("Provider/Theme = %q/%q, want openai/nord", out.Provider, out.Theme)
	}
	if out.Browser.Command != "/usr/bin/firefox" {
		t.Errorf("Browser.Command = %q, want /usr/bin/firefox", out.Browser.Command)
	}
	got := out.Providers["openai"]
	if got.APIKey != "sk-test-123" || got.BaseURL != "https://api.openai.com/v1" || got.Model != "gpt-4o-mini" {
		t.Errorf("Providers[openai] = %+v, want stored credentials", got)
	}
	if out.Providers["zai"].APIKey != "zkey" {
		t.Errorf("Providers[zai].APIKey = %q, want zkey", out.Providers["zai"].APIKey)
	}
	if out.Permissions.Write != "ask" || out.Permissions.Browser != "deny" {
		t.Errorf("Permissions = %+v, want stored policies", out.Permissions)
	}
}

// Regression: before mapstructure tags existed, YAML keys with underscores
// (base_url, api_key_env) were silently dropped by Load.
func TestLoadReadsLegacyFlatKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := "base_url: http://legacy.example/v1\napi_key_env: LEGACY_KEY\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BaseURL != "http://legacy.example/v1" {
		t.Errorf("BaseURL = %q, want legacy value loaded", cfg.BaseURL)
	}
	if cfg.APIKeyEnv != "LEGACY_KEY" {
		t.Errorf("APIKeyEnv = %q, want LEGACY_KEY", cfg.APIKeyEnv)
	}
}

func TestDefaultPathIsUnderConfigDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	p, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	if want := filepath.Join(home, ".tharness", "config.yaml"); p != want {
		t.Errorf("DefaultPath = %q, want %q", p, want)
	}
}
