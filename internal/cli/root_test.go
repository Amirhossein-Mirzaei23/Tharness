package cli

import "testing"

func TestNewRootCommand(t *testing.T) {
	// Stub the TUI seam: bare invocation must not launch a real terminal
	// program in tests.
	prev := tuiStarter
	tuiStarter = func() error { return nil }
	t.Cleanup(func() { tuiStarter = prev })

	root := NewRootCommand()
	if root == nil {
		t.Fatal("NewRootCommand() returned nil")
	}
	if root.Use != "tharness" {
		t.Errorf("Use = %q, want %q", root.Use, "tharness")
	}
	if root.Version == "" {
		t.Error("Version is empty; want a version placeholder")
	}
	if err := root.Execute(); err != nil {
		t.Errorf("Execute() with no args returned error: %v", err)
	}
	configFlag := root.Flags().Lookup("config")
	if configFlag == nil {
		t.Fatal("--config flag is not registered")
	}
	if configFlag.DefValue != "" {
		t.Errorf("--config default = %q, want empty", configFlag.DefValue)
	}
}
