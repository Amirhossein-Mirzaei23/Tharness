package cli

import (
	"bytes"
	"testing"
)

// stubTUIStarter records invocation and does nothing (no terminal needed).
func stubTUIStarter(t *testing.T) *[]string {
	t.Helper()
	var calls []string
	prev := tuiStarter
	tuiStarter = func() error {
		calls = append(calls, "chat")
		return nil
	}
	t.Cleanup(func() { tuiStarter = prev })
	return &calls
}

// RED: bare `tabdl` (no args) starts the TUI via the seam, not help.
func TestBareRootStartsTUI(t *testing.T) {
	calls := stubTUIStarter(t)
	root := NewRootCommand()
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() bare: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("tuiStarter calls = %d, want 1", len(*calls))
	}
}

// RED: `tabdl chat` starts the TUI via the seam.
func TestChatSubcommandStartsTUI(t *testing.T) {
	calls := stubTUIStarter(t)
	root := NewRootCommand()
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{"chat"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() chat: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("tuiStarter calls = %d, want 1", len(*calls))
	}
}
