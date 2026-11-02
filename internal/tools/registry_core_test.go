package tools

import "testing"

func TestNewCoreRegistryIncludesEdit(t *testing.T) {
	reg := NewCoreRegistry(t.TempDir())
	if _, ok := reg.Get("edit"); !ok {
		t.Fatal("NewCoreRegistry should register edit")
	}
}
