package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootCommandOneShotPromptFlag(t *testing.T) {
	srv := seqServer(t, textSSE)

	dataDir := t.TempDir()
	t.Setenv("THARNESS_BASE_URL", srv.URL)
	t.Setenv("THARNESS_API_KEY", "test-key")
	t.Setenv("THARNESS_DATA_DIR", dataDir)

	stdout := &bytes.Buffer{}
	root := NewRootCommand()
	root.SetOut(stdout)
	root.SetArgs([]string{"-p", "say hello"})
	if err := root.Execute(); err != nil {
		t.Fatalf("root.Execute() error: %v", err)
	}

	if got := strings.TrimSpace(stdout.String()); got != "Hello world" {
		t.Fatalf("stdout = %q, want %q", got, "Hello world")
	}
	entries, err := os.ReadDir(filepath.Join(dataDir, "sessions"))
	if err != nil {
		t.Fatalf("sessions dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d session files, want 1", len(entries))
	}
	lines := readTranscript(t, filepath.Join(dataDir, "sessions", entries[0].Name()))
	if len(lines) < 2 {
		t.Fatalf("transcript has %d lines, want >= 2", len(lines))
	}
}

func TestRunSubcommandPromptFlag(t *testing.T) {
	srv := seqServer(t, textSSE)

	dataDir := t.TempDir()
	t.Setenv("THARNESS_BASE_URL", srv.URL)
	t.Setenv("THARNESS_API_KEY", "test-key")
	t.Setenv("THARNESS_DATA_DIR", dataDir)

	stdout := &bytes.Buffer{}
	root := NewRootCommand()
	root.SetOut(stdout)
	root.SetArgs([]string{"run", "-p", "say hello"})
	if err := root.Execute(); err != nil {
		t.Fatalf("root.Execute(run -p) error: %v", err)
	}

	if got := strings.TrimSpace(stdout.String()); got != "Hello world" {
		t.Fatalf("stdout = %q, want %q", got, "Hello world")
	}
}
