package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteOverwritesExistingFile(t *testing.T) {
	ws := newTestWorkspace(t)
	writeFixture(t, ws, "existing.txt", "old content")
	tool := NewWriteTool(ws, WriteOptions{})

	out, err := tool.Exec(context.Background(), `{"path":"existing.txt","content":"new"}`)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(ws, "existing.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("got %q, want %q", data, "new")
	}
	if !strings.Contains(out, "3 bytes") {
		t.Fatalf("confirmation should mention 3 bytes, got: %s", out)
	}
}

func TestWriteRefusesPathOutsideWorkspace(t *testing.T) {
	ws := newTestWorkspace(t)
	tool := NewWriteTool(ws, WriteOptions{})

	_, err := tool.Exec(context.Background(), `{"path":"../../escape.txt","content":"nope"}`)
	if err == nil {
		t.Fatal("expected error for path outside workspace")
	}
	if !strings.Contains(err.Error(), "outside the workspace") {
		t.Fatalf("error should mention outside workspace, got: %v", err)
	}
	// The escape target (ws/../../escape.txt) must not exist.
	if _, statErr := os.Stat(filepath.Join(ws, "..", "..", "escape.txt")); statErr == nil {
		t.Fatal("file must NOT be created outside workspace")
	}
}

func TestWriteAllowOutsideWorkspaceOption(t *testing.T) {
	ws := newTestWorkspace(t)
	tool := NewWriteTool(ws, WriteOptions{AllowOutsideWorkspace: true})

	out, err := tool.Exec(context.Background(), `{"path":"../sibling.txt","content":"allowed"}`)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(ws, "..", "sibling.txt"))
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if string(data) != "allowed" {
		t.Fatalf("got %q, want %q", data, "allowed")
	}
	if !strings.Contains(out, "sibling.txt") {
		t.Fatalf("confirmation should mention path, got: %s", out)
	}
}

func TestWriteCreatesFileAndParentDirs(t *testing.T) {
	ws := newTestWorkspace(t)
	tool := NewWriteTool(ws, WriteOptions{})

	out, err := tool.Exec(context.Background(), `{"path":"a/b/c.txt","content":"hello world"}`)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(ws, "a", "b", "c.txt"))
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if string(data) != "hello world" {
		t.Fatalf("got %q, want %q", data, "hello world")
	}
	if !strings.Contains(out, "11 bytes") {
		t.Fatalf("confirmation should mention byte count, got: %s", out)
	}
}
