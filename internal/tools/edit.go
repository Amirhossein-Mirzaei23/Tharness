package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EditTool replaces an exact, unique occurrence of a string inside a
// workspace file.
type EditTool struct {
	root string
	opts EditOptions
}

// EditOptions configures the edit tool.
type EditOptions struct {
	// AllowOutsideWorkspace permits paths that escape the workspace root.
	// Default false: escapes are refused.
	AllowOutsideWorkspace bool
}

// NewEditTool returns an EditTool rooted at the absolute workspace dir.
func NewEditTool(root string, opts EditOptions) *EditTool {
	return &EditTool{root: root, opts: opts}
}

func (t *EditTool) Name() string { return "edit" }

func (t *EditTool) Description() string {
	return "Replace an exact, unique occurrence of old_string with new_string in a file. old_string must appear exactly once."
}

func (t *EditTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":       map[string]any{"type": "string", "description": "Path relative to the workspace root"},
			"old_string": map[string]any{"type": "string", "description": "Exact text to replace (must be unique in the file)"},
			"new_string": map[string]any{"type": "string", "description": "Replacement text"},
		},
		"required": []string{"path", "old_string", "new_string"},
	}
}

func (t *EditTool) Exec(_ context.Context, argsJSON string) (string, error) {
	var args struct {
		Path      string `json:"path"`
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("edit: parse args: %w", err)
	}
	if args.Path == "" {
		return "", fmt.Errorf("edit: path is required")
	}
	if args.OldString == args.NewString {
		return "", fmt.Errorf("edit: old_string and new_string are identical; nothing to change")
	}
	abs, err := t.resolve(args.Path)
	if err != nil {
		return "", fmt.Errorf("edit: %w", err)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", fmt.Errorf("edit %s: %w", args.Path, err)
	}
	content := string(data)
	count := countOccurrences(content, args.OldString)
	switch {
	case count == 0:
		return "", fmt.Errorf("edit %s: old_string not found", args.Path)
	case count > 1:
		return "", fmt.Errorf("edit %s: old_string not unique (%d occurrences); add surrounding context to make it unique", args.Path, count)
	}
	updated := replaceOnce(content, args.OldString, args.NewString)
	if err := os.WriteFile(abs, []byte(updated), 0o644); err != nil {
		return "", fmt.Errorf("edit %s: %w", args.Path, err)
	}
	return fmt.Sprintf("edited %s\n%s", args.Path, diffPreview(content, updated)), nil
}

// resolve mirrors WriteTool.resolve: cleans path and joins it under root.
// Unless AllowOutsideWorkspace is set, relative escapes (../..) and absolute
// paths are refused.
func (t *EditTool) resolve(path string) (string, error) {
	if t.opts.AllowOutsideWorkspace {
		if filepath.IsAbs(path) {
			return filepath.Clean(path), nil
		}
		return filepath.Join(t.root, filepath.Clean(path)), nil
	}
	if !filepath.IsLocal(path) {
		return "", fmt.Errorf("path %q is outside the workspace", path)
	}
	return filepath.Join(t.root, filepath.Clean(path)), nil
}

// countOccurrences counts non-overlapping occurrences of needle in s.
func countOccurrences(s, needle string) int {
	if needle == "" {
		return 0
	}
	return strings.Count(s, needle)
}

// replaceOnce replaces the first (and by guard, only) occurrence.
func replaceOnce(s, old, new string) string {
	return strings.Replace(s, old, new, 1)
}

// diffPreview returns a simple unified-diff-style preview: context lines
// around each change with -/+ markers.
func diffPreview(oldContent, newContent string) string {
	oldLines := strings.Split(strings.TrimRight(oldContent, "\n"), "\n")
	newLines := strings.Split(strings.TrimRight(newContent, "\n"), "\n")
	var b strings.Builder
	// Simple line-by-line diff: mark lines removed/added at the first
	// divergence, then shared trailing lines as context.
	i := 0
	for i < len(oldLines) && i < len(newLines) && oldLines[i] == newLines[i] {
		fmt.Fprintf(&b, "  %s\n", oldLines[i])
		i++
	}
	j := len(oldLines)
	k := len(newLines)
	for j > i && k > i && oldLines[j-1] == newLines[k-1] {
		j--
		k--
	}
	for x := i; x < j; x++ {
		fmt.Fprintf(&b, "- %s\n", oldLines[x])
	}
	for x := i; x < k; x++ {
		fmt.Fprintf(&b, "+ %s\n", newLines[x])
	}
	for x := j; x < len(oldLines); x++ {
		fmt.Fprintf(&b, "  %s\n", oldLines[x])
	}
	return strings.TrimRight(b.String(), "\n")
}
