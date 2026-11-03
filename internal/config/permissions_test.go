package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Amirhossein-Mirzaei23/Tharness/internal/permissions"
)

// RED: permissions.* YAML keys parse into Config.Permissions.
func TestLoadParsesPermissionsPolicies(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := "permissions:\n  bash: deny\n  write: ask\n"
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load(%q) returned error: %v", path, err)
	}
	if cfg.Permissions.Bash != "deny" {
		t.Errorf("Permissions.Bash = %q, want %q", cfg.Permissions.Bash, "deny")
	}
	if cfg.Permissions.Write != "ask" {
		t.Errorf("Permissions.Write = %q, want %q", cfg.Permissions.Write, "ask")
	}
}

// RED: an empty config yields "auto" defaults for every gated tool.
func TestLoadPermissionsDefaultsToAuto(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("model: m\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load(%q) returned error: %v", path, err)
	}
	if cfg.Permissions.Bash != "auto" || cfg.Permissions.Write != "auto" || cfg.Permissions.Edit != "auto" {
		t.Errorf("Permissions = %+v, want auto defaults for bash/write/edit", cfg.Permissions)
	}
}

// RED: BuildGate maps config policies onto a permissions.Gate.
func TestBuildGateFromConfig(t *testing.T) {
	gate := BuildGate(Permissions{Bash: "deny", Write: "ask", Edit: "auto"})

	if got := gate.Check("bash"); got != permissions.DecisionDeny {
		t.Errorf("gate.Check(bash) = %v, want %v", got, permissions.DecisionDeny)
	}
	if got := gate.Check("write"); got != permissions.DecisionAsk {
		t.Errorf("gate.Check(write) = %v, want %v", got, permissions.DecisionAsk)
	}
	if got := gate.Check("edit"); got != permissions.DecisionAllow {
		t.Errorf("gate.Check(edit) = %v, want %v", got, permissions.DecisionAllow)
	}
}
