package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadTool reads a file from inside the workspace and returns its content
// with line numbers, truncated at a maximum number of lines.
type ReadTool struct {
	root string
	opts ReadOptions
}

// ReadOptions configures the read tool.
type ReadOptions struct {
	// MaxLines truncates output after this many lines (default 2000).
	MaxLines int
}

// NewReadTool returns a ReadTool rooted at the absolute workspace dir.
func NewReadTool(root string, opts ReadOptions) *ReadTool {
	if opts.MaxLines <= 0 {
		opts.MaxLines = 2000
	}
	return &ReadTool{root: root, opts: opts}
}

func (t *ReadTool) Name() string { return "read" }

func (t *ReadTool) Description() string {
	return "Read a file from the workspace. Returns the content with line numbers, truncated at 2000 lines."
}

func (t *ReadTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "Path relative to the workspace root"},
		},
		"required": []string{"path"},
	}
}

func (t *ReadTool) Exec(_ context.Context, argsJSON string) (string, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("read: parse args: %w", err)
	}
	abs, err := resolveInWorkspace(t.root, args.Path)
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}
	f, err := os.Open(abs)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", args.Path, err)
	}
	defer f.Close() //nolint:errcheck // read-only closer

	var b strings.Builder
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	line := 0
	truncated := false
	for sc.Scan() {
		line++
		if line > t.opts.MaxLines {
			truncated = true
			break
		}
		fmt.Fprintf(&b, "%4d|\t%s\n", line, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("read %s: %w", args.Path, err)
	}
	out := strings.TrimRight(b.String(), "\n")
	if truncated {
		out += fmt.Sprintf("\n[truncated: showing first %d of more lines]", t.opts.MaxLines)
	}
	return out, nil
}

// resolveInWorkspace cleans path and joins it under root, rejecting
// relative escapes such as "../.." that leave the workspace.
func resolveInWorkspace(root, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if !filepath.IsLocal(path) {
		return "", fmt.Errorf("path %q is outside the workspace", path)
	}
	return filepath.Join(root, filepath.Clean(path)), nil
}
