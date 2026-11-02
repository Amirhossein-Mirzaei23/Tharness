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
	if cfg.Model != "zai/glm-5.3-flash" {
		t.Errorf("Model = %q, want default %q", cfg.Model, "zai/glm-5.3-flash")
	}
	if cfg.DataDir != filepath.Join(home, ".tabdl") {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, filepath.Join(home, ".tabdl"))
	}
}
