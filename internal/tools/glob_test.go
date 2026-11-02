package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// globFixture builds a small tree:
//
//	a/b/x.go
//	y.go
//	skip/.git/ign.go     (.git must never be walked)
//	node_modules/dep.js  (covered by .gitignore written by caller when wanted)
func globFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"a/b/x.go":                        "package b\n",
		"y.go":                            "package main\n",
		filepath.Join("skip/.git/ign.go"): "package ign\n",
		"node_modules/dep.js":             "// dep\n",
		"a/b/readme.md":                   "# md\n",
	}
	for rel, content := range files {
		abs := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestGlobToolCapsAtMaxMatches(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 600; i++ {
		abs := filepath.Join(dir, fmt.Sprintf("f%03d.txt", i))
		if err := os.WriteFile(abs, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tool := NewGlobTool(dir, GlobOptions{})

	out, err := tool.Exec(context.Background(), `{"pattern": "**/*.txt"}`)
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 501 {
		t.Fatalf("got %d lines, want 500 matches + 1 note", len(lines))
	}
	if !strings.Contains(lines[500], "truncated") || !strings.Contains(lines[500], "500") {
		t.Errorf("last line = %q, want truncation note mentioning 500", lines[500])
	}
}

func TestGlobToolRespectsRootGitignore(t *testing.T) {
	dir := globFixture(t)
	// Ignore node_modules (dir rule) and all .log files anywhere.
	gitignoreSrc := "node_modules/\n*.log\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(gitignoreSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "build", "out.log"), []byte("log"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := NewGlobTool(dir, GlobOptions{})

	out, err := tool.Exec(context.Background(), `{"pattern": "**/*"}`)
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
	if strings.Contains(out, "node_modules") {
		t.Errorf("gitignored node_modules leaked into results:\n%s", out)
	}
	if strings.Contains(out, ".log") {
		t.Errorf("gitignored .log file leaked into results:\n%s", out)
	}
	// Sanity: non-ignored files are still present.
	if !strings.Contains(out, "a/b/x.go") || !strings.Contains(out, "y.go") {
		t.Errorf("expected non-ignored files in results:\n%s", out)
	}
}

func TestGlobToolDoublestarGoFiles(t *testing.T) {
	dir := globFixture(t)
	tool := NewGlobTool(dir, GlobOptions{})

	out, err := tool.Exec(context.Background(), `{"pattern": "**/*.go"}`)
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}

	got := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"a/b/x.go", "y.go"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("match[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
