package tools

import (
	"strings"
	"testing"
)

func TestNewCoreRegistryIncludesReadWrite(t *testing.T) {
	reg := NewCoreRegistry(t.TempDir())
	names := reg.Names()
	joined := strings.Join(names, ",")
	for _, want := range []string{"read", "write"} {
		if !strings.Contains(joined, want) {
			t.Errorf("NewCoreRegistry().Names() = %v, missing %q", names, want)
		}
	}
}

func TestNewCoreRegistryIncludesGlobAndGrep(t *testing.T) {
	reg := NewCoreRegistry(t.TempDir())
	names := reg.Names()
	joined := strings.Join(names, ",")
	for _, want := range []string{"glob", "grep"} {
		if !strings.Contains(joined, want) {
			t.Errorf("NewCoreRegistry().Names() = %v, missing %q", names, want)
		}
	}
	if _, ok := reg.Get("glob"); !ok {
		t.Error("glob not registered")
	}
	if _, ok := reg.Get("grep"); !ok {
		t.Error("grep not registered")
	}
}
