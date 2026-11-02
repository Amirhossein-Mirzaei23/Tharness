package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestWorkspace creates a temp workspace and returns it.
func newTestWorkspace(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func writeFixture(t *testing.T, ws, rel, content string) {
	t.Helper()
	path := filepath.Join(ws, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadTruncatesLongFile(t *testing.T) {
	ws := newTestWorkspace(t)
	lines := make([]string, 2500)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i+1)
	}
	writeFixture(t, ws, "big.txt", strings.Join(lines, "\n")+"\n")

	tool := NewReadTool(ws, ReadOptions{})
	out, err := tool.Exec(context.Background(), `{"path":"big.txt"}`)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if !strings.Contains(out, "2000|	line 2000") {
		t.Fatalf("line 2000 missing from output:\n%s", tailOf(out, 200))
	}
	if strings.Contains(out, "line 2001") {
		t.Fatal("output should not contain line 2001")
	}
	if !strings.Contains(out, "[truncated") {
		t.Fatal("missing truncation marker")
	}
}

func tailOf(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

func TestReadMissingFileErrors(t *testing.T) {
	ws := newTestWorkspace(t)
	tool := NewReadTool(ws, ReadOptions{})
	_, err := tool.Exec(context.Background(), `{"path":"nope.txt"}`)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadRejectsPathOutsideWorkspace(t *testing.T) {
	ws := newTestWorkspace(t)
	tool := NewReadTool(ws, ReadOptions{})
	_, err := tool.Exec(context.Background(), `{"path":"../../etc/passwd"}`)
	if err == nil {
		t.Fatal("expected error for path outside workspace")
	}
	if !strings.Contains(err.Error(), "outside the workspace") {
		t.Fatalf("error should mention outside workspace, got: %v", err)
	}
}

func TestReadReturnsNumberedContent(t *testing.T) {
	ws := newTestWorkspace(t)
	writeFixture(t, ws, "hello.txt", "alpha\nbeta\ngamma\n")

	tool := NewReadTool(ws, ReadOptions{})
	out, err := tool.Exec(context.Background(), `{"path":"hello.txt"}`)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	want := "   1|\talpha\n   2|\tbeta\n   3|\tgamma"
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
	if !strings.Contains(out, "beta") {
		t.Fatal("content missing")
	}
}
