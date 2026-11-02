package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/amir/tabdl/internal/session"
)

func TestSessionsListPrintsIDs(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("TABDL_DATA_DIR", dataDir)

	store, err := session.Open(dataDir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	id, path, err := store.New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if err := store.Append(path, session.Event{TS: time.Now().UTC().Format(time.RFC3339), Type: "message", Role: "user", Content: "hi"}); err != nil {
		t.Fatalf("Append() error: %v", err)
	}

	root := NewRootCommand()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetArgs([]string{"sessions", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if !strings.Contains(out.String(), id) {
		t.Fatalf("sessions list output %q does not contain id %q", out.String(), id)
	}
}

func TestSessionsListEmptyDirOK(t *testing.T) {
	t.Setenv("TABDL_DATA_DIR", t.TempDir())

	root := NewRootCommand()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetArgs([]string{"sessions", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if s := strings.TrimSpace(out.String()); s != "" && !strings.HasPrefix(s, "no sessions") {
		t.Fatalf("unexpected output for empty dir: %q", s)
	}
}
