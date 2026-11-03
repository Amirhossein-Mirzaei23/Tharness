package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromYAMLFileSetsModel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := "model: zai/glm-5.3-flash\n"
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load(%q) returned error: %v", path, err)
	}
	if cfg.Model != "zai/glm-5.3-flash" {
		t.Errorf("Model = %q, want %q", cfg.Model, "zai/glm-5.3-flash")
	}
}

func TestLoadDefaultsWhenFileAbsent(t *testing.T) {
	home := t.TempDir() // overridden home keeps the test isolated from the real $HOME
	t.Setenv("HOME", home)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\") returned error: %v", err)
	}
	if cfg.Model != "" {
		t.Errorf("Model = %q, want empty (model resolution happens in ai.ResolveProvider)", cfg.Model)
	}
	if cfg.Provider != DefaultProvider {
		t.Errorf("Provider = %q, want default %q", cfg.Provider, DefaultProvider)
	}
	if cfg.Theme != DefaultTheme {
		t.Errorf("Theme = %q, want default %q", cfg.Theme, DefaultTheme)
	}
	if cfg.Permissions.Browser != "auto" {
		t.Errorf("Permissions.Browser = %q, want auto", cfg.Permissions.Browser)
	}
	if cfg.DataDir != filepath.Join(home, ".tharness") {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, filepath.Join(home, ".tharness"))
	}
}
