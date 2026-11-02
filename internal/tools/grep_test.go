package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// grepFixture builds a tree for grep tests:
//
//	code/main.go   (two lines containing "func ")
//	code/util.go   (one line containing "func ")
//	docs/notes.md  (mentions func in prose)
//	bin/data.bin   (NUL byte in first 512 bytes + "func " on line 2)
//	junk/skip.go   (gitignored when caller writes .gitignore)
func grepFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"code/main.go":  "package main\nfunc main() {}\n// no match here\nfunc helper() {}\n",
		"code/util.go":  "package code\nfunc util() {}\n",
		"docs/notes.md": "# notes\nthe func keyword is nice\n",
		"junk/skip.go":  "func skipped() {}\n",
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
	// Binary-ish file: NUL in the first 512 bytes, plus a matchable line.
	bin := filepath.Join(dir, "bin", "data.bin")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	binContent := append([]byte("func \x00binary"), make([]byte, 500)...)
	binContent = append(binContent, []byte("\nfunc later() {}\n")...)
	if err := os.WriteFile(bin, binContent, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestGrepToolCapsAt200WithNote(t *testing.T) {
	dir := t.TempDir()
	// One file, 300 matching lines -> cap at 200 with note.
	abs := filepath.Join(dir, "many.txt")
	var b strings.Builder
	for i := 0; i < 300; i++ {
		fmt.Fprintf(&b, "needle %d\n", i)
	}
	if err := os.WriteFile(abs, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := NewGrepTool(dir, GrepOptions{})

	out, err := tool.Exec(context.Background(), `{"pattern": "needle"}`)
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 201 {
		t.Fatalf("got %d lines, want 200 matches + 1 note", len(lines))
	}
	if !strings.Contains(lines[200], "truncated") || !strings.Contains(lines[200], "200") {
		t.Errorf("last line = %q, want truncation note mentioning 200", lines[200])
	}
}

func TestGrepToolSkipsBinaryFiles(t *testing.T) {
	dir := grepFixture(t)
	tool := NewGrepTool(dir, GrepOptions{})

	// data.bin has a NUL in its first 512 bytes and a matching line 2.
	out, err := tool.Exec(context.Background(), `{"pattern": "func later"}`)
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
	if strings.TrimSpace(out) != "no matches" {
		t.Errorf("binary file was scanned: %s", out)
	}
}

func TestGrepToolGlobFilterOnlyScansMd(t *testing.T) {
	dir := grepFixture(t)
	tool := NewGrepTool(dir, GrepOptions{})

	out, err := tool.Exec(context.Background(), `{"pattern": "func ", "glob": "*.md"}`)
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}

	got := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"docs/notes.md:2: the func keyword is nice"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestGrepToolIgnoreCase(t *testing.T) {
	dir := grepFixture(t)
	// fixture has no uppercase FUNC; add one case variant file.
	abs := filepath.Join(dir, "code", "case.go")
	if err := os.WriteFile(abs, []byte("package code\nFUNC upper() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := NewGrepTool(dir, GrepOptions{})

	// Case-sensitive: no match in case.go.
	out, err := tool.Exec(context.Background(), `{"pattern": "func ", "glob": "case.go"}`)
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
	if strings.Contains(out, "case.go") {
		t.Errorf("case-sensitive search matched case.go: %s", out)
	}

	// ignore_case: matches.
	out, err = tool.Exec(context.Background(), `{"pattern": "func ", "glob": "case.go", "ignore_case": true}`)
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
	if !strings.Contains(out, "code/case.go:2: FUNC upper() {}") {
		t.Errorf("ignore_case did not match case.go: %s", out)
	}
}

func TestGrepToolInvalidRegexpReturnsError(t *testing.T) {
	dir := grepFixture(t)
	tool := NewGrepTool(dir, GrepOptions{})

	_, err := tool.Exec(context.Background(), `{"pattern": "func("}`)
	if err == nil {
		t.Fatal("expected error for invalid regexp, got nil")
	}
}

func TestGrepToolBasicMatches(t *testing.T) {
	dir := grepFixture(t)
	tool := NewGrepTool(dir, GrepOptions{})

	out, err := tool.Exec(context.Background(), `{"pattern": "func "}`)
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}

	want := []string{
		"code/main.go:2: func main() {}",
		"code/main.go:4: func helper() {}",
		"code/util.go:2: func util() {}",
		"docs/notes.md:2: the func keyword is nice",
		"junk/skip.go:1: func skipped() {}",
	}
	got := strings.Split(strings.TrimSpace(out), "\n")
	if len(got) != len(want) {
		t.Fatalf("got %d lines %v, want %v", len(got), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
