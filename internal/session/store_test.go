package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesSessionsDir(t *testing.T) {
	dir := t.TempDir()

	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	if s == nil {
		t.Fatal("Open() returned nil store")
	}

	info, err := os.Stat(filepath.Join(dir, "sessions"))
	if err != nil {
		t.Fatalf("sessions dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("sessions path is not a directory")
	}
}

func TestNewGivesUniqueIDs(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	id1, path1, err := s.New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	id2, path2, err := s.New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if id1 == id2 {
		t.Fatalf("ids not unique: %q == %q", id1, id2)
	}
	if path1 == path2 {
		t.Fatalf("paths not unique: %q == %q", path1, path2)
	}
	if filepath.Base(path1) != id1+".jsonl" {
		t.Fatalf("path base = %q, want %q.jsonl", filepath.Base(path1), id1)
	}
}
