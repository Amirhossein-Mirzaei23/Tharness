package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WriteTool creates or overwrites a file inside the workspace, creating
// parent directories as needed.
type WriteTool struct {
	root string
	opts WriteOptions
}

// WriteOptions configures the write tool.
type WriteOptions struct {
	// AllowOutsideWorkspace permits paths that escape the workspace root.
	// Default false: escapes are refused.
	AllowOutsideWorkspace bool
}

// NewWriteTool returns a WriteTool rooted at the absolute workspace dir.
func NewWriteTool(root string, opts WriteOptions) *WriteTool {
	return &WriteTool{root: root, opts: opts}
}

func (t *WriteTool) Name() string { return "write" }

func (t *WriteTool) Description() string {
	return "Create or overwrite a file in the workspace. Parent directories are created automatically."
}

func (t *WriteTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":    map[string]any{"type": "string", "description": "Path relative to the workspace root"},
			"content": map[string]any{"type": "string", "description": "Full file content to write"},
		},
		"required": []string{"path", "content"},
	}
}

func (t *WriteTool) Exec(_ context.Context, argsJSON string) (string, error) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("write: parse args: %w", err)
	}
	if args.Path == "" {
		return "", fmt.Errorf("write: path is required")
	}
	abs, err := t.resolve(args.Path)
	if err != nil {
		return "", fmt.Errorf("write: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", fmt.Errorf("write %s: %w", args.Path, err)
	}
	if err := os.WriteFile(abs, []byte(args.Content), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", args.Path, err)
	}
	return fmt.Sprintf("wrote %s (%d bytes)", args.Path, len(args.Content)), nil
}

// resolve cleans path and joins it under root. Unless AllowOutsideWorkspace
// is set, relative escapes (../..) and absolute paths are refused.
func (t *WriteTool) resolve(path string) (string, error) {
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
