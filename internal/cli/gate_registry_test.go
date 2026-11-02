package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/amir/tabdl/internal/permissions"
	"github.com/amir/tabdl/internal/tools"
)

// RED: GateRegistry wraps bash/write/edit in GatedTool per the Gate's
// policies; a denied tool errors at Exec time and others are untouched.
func TestGateRegistryAppliesGateToGatedTools(t *testing.T) {
	dir := t.TempDir()
	reg := tools.NewCoreRegistry(dir)
	gate := permissions.NewGate(map[string]permissions.Policy{
		"bash": permissions.PolicyDeny,
	})
	permissions.GateRegistry(reg, gate, nil)
	// bash is now gated and denied without executing.
	bash, ok := reg.Get("bash")
	if !ok {
		t.Fatalf("registry has no bash tool")
	}
	_, err := bash.Exec(context.Background(), `{}`)
	if err == nil || !strings.Contains(err.Error(), "permission denied by policy (bash)") {
		t.Errorf("bash Exec err = %v, want policy denial", err)
	}

	// read is not gated: it executes normally (missing file error, not a
	// permission error).
	read, _ := reg.Get("read")
	_, err = read.Exec(context.Background(), `{"path":"nope.txt"}`)
	if err != nil && strings.Contains(err.Error(), "permission denied by policy") {
		t.Errorf("read Exec err = %v, want un-gated execution", err)
	}
}
