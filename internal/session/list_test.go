package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestListReturnsIDAndLineCount(t *testing.T) {
	s := mustStore(t)
	id, path, err := s.New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	ts := time.Date(2026, 11, 2, 15, 4, 5, 0, time.UTC).Format(time.RFC3339)
	for _, ev := range []Event{
		{TS: ts, Type: "message", Role: "user", Content: "one"},
		{TS: ts, Type: "message", Role: "assistant", Content: "two"},
	} {
		if err := s.Append(path, ev); err != nil {
			t.Fatalf("Append() error: %v", err)
		}
	}

	sessions, err := s.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("List returned %d sessions, want 1", len(sessions))
	}
	got := sessions[0]
	if got.ID != id {
		t.Errorf("ID = %q, want %q", got.ID, id)
	}
	if got.Path != path {
		t.Errorf("Path = %q, want %q", got.Path, path)
	}
	if got.LineCount != 2 {
		t.Errorf("LineCount = %d, want 2", got.LineCount)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
}

func TestListIgnoresNonJSONLFiles(t *testing.T) {
	s := mustStore(t)
	id, path, err := s.New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if err := s.Append(path, Event{TS: "t", Type: "message", Role: "user"}); err != nil {
		t.Fatalf("Append() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(s.dir, "notes.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("write stray file: %v", err)
	}

	sessions, err := s.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != id {
		t.Fatalf("List = %+v, want only %q", sessions, id)
	}
}

func TestLoadToleratesTrailingNewlineAndCorruptLine(t *testing.T) {
	s := mustStore(t)
	id, path, err := s.New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	ts := time.Date(2026, 11, 2, 15, 4, 5, 0, time.UTC).Format(time.RFC3339)
	if err := s.Append(path, Event{TS: ts, Type: "message", Role: "user", Content: "fine"}); err != nil {
		t.Fatalf("Append() error: %v", err)
	}
	// Corrupt line + trailing newline, written directly to simulate damage.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open transcript: %v", err)
	}
	if _, err := f.WriteString("{not json\n\n"); err != nil {
		t.Fatalf("write corrupt line: %v", err)
	}
	f.Close() //nolint:errcheck // test cleanup

	got, warnings, err := s.Load(id)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(got) != 1 || got[0].Content != "fine" {
		t.Fatalf("Load = %+v, want the single fine event", got)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly 1", warnings)
	}
}
