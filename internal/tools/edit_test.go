package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditReplacesUniqueOccurrence(t *testing.T) {
	ws := newTestWorkspace(t)
	writeFixture(t, ws, "code.txt", "alpha\nbeta\ngamma\n")
	tool := NewEditTool(ws, EditOptions{})

	out, err := tool.Exec(context.Background(), `{"path":"code.txt","old_string":"beta","new_string":"BETA"}`)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(ws, "code.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "alpha\nBETA\ngamma\n" {
		t.Fatalf("file not updated, got %q", data)
	}
	if !strings.Contains(out, "BETA") {
		t.Fatalf("result should mention the change, got: %s", out)
	}
}

func TestEditOldStringNotFound(t *testing.T) {
	ws := newTestWorkspace(t)
	writeFixture(t, ws, "code.txt", "alpha\nbeta\n")
	tool := NewEditTool(ws, EditOptions{})

	_, err := tool.Exec(context.Background(), `{"path":"code.txt","old_string":"missing","new_string":"x"}`)
	if err == nil {
		t.Fatal("expected error for zero occurrences")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error should mention not found, got: %v", err)
	}
}

func TestEditOldStringNotUniqueLeavesFileUnchanged(t *testing.T) {
	ws := newTestWorkspace(t)
	original := "dup here\nmiddle\ndup here\n"
	writeFixture(t, ws, "code.txt", original)
	tool := NewEditTool(ws, EditOptions{})

	_, err := tool.Exec(context.Background(), `{"path":"code.txt","old_string":"dup here","new_string":"replaced"}`)
	if err == nil {
		t.Fatal("expected error for multiple occurrences")
	}
	if !strings.Contains(err.Error(), "not unique") {
		t.Fatalf("error should mention not unique, got: %v", err)
	}
	if !strings.Contains(err.Error(), "2") {
		t.Fatalf("error should mention occurrence count, got: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(ws, "code.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatalf("file must be unchanged on failure, got %q", data)
	}
}

func TestEditRefusesPathOutsideWorkspace(t *testing.T) {
	ws := newTestWorkspace(t)
	writeFixture(t, ws, "code.txt", "alpha\nbeta\n")
	tool := NewEditTool(ws, EditOptions{})

	_, err := tool.Exec(context.Background(), `{"path":"../../escape.txt","old_string":"beta","new_string":"BETA"}`)
	if err == nil {
		t.Fatal("expected error for path outside workspace")
	}
	if !strings.Contains(err.Error(), "outside the workspace") {
		t.Fatalf("error should mention outside workspace, got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(ws, "..", "..", "escape.txt")); statErr == nil {
		t.Fatal("file must NOT be touched outside workspace")
	}
}

func TestEditMissingFile(t *testing.T) {
	ws := newTestWorkspace(t)
	tool := NewEditTool(ws, EditOptions{})

	_, err := tool.Exec(context.Background(), `{"path":"nope.txt","old_string":"a","new_string":"b"}`)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "nope.txt") {
		t.Fatalf("error should mention path, got: %v", err)
	}
}

func TestEditRefusesNoOpEdit(t *testing.T) {
	ws := newTestWorkspace(t)
	original := "alpha\nbeta\n"
	writeFixture(t, ws, "code.txt", original)
	tool := NewEditTool(ws, EditOptions{})

	_, err := tool.Exec(context.Background(), `{"path":"code.txt","old_string":"beta","new_string":"beta"}`)
	if err == nil {
		t.Fatal("expected error for identical old/new strings")
	}
	data, err := os.ReadFile(filepath.Join(ws, "code.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatalf("file must be unchanged, got %q", data)
	}
}
