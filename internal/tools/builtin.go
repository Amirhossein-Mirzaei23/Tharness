// Package tools: built-in core tool registry.
package tools

import "path/filepath"

// NewCoreRegistry returns the registry of core coding tools available to the
// agent loop, rooted at the workspace dir. Tasks 5+ register the real set
// (read, write, edit, bash, glob, grep, todo) here. The workspace root is
// optional for now (variadic) so existing no-arg callers keep compiling;
// tools constructed without a root fall back to the process working dir.
func NewCoreRegistry(root ...string) *Registry {
	dir := ""
	if len(root) > 0 {
		dir = root[0]
	}
	if dir == "" {
		if wd, err := absoluteWorkdir(); err == nil {
			dir = wd
		}
	}
	reg := NewRegistry()
	reg.Register(NewReadTool(dir, ReadOptions{}))
	reg.Register(NewWriteTool(dir, WriteOptions{}))
	reg.Register(NewEditTool(dir, EditOptions{}))
	reg.Register(NewBashTool(dir, BashOptions{}))
	reg.Register(NewGlobTool(dir, GlobOptions{}))
	reg.Register(NewGrepTool(dir, GrepOptions{}))
	reg.Register(NewTodoTool(filepath.Join(dir, ".tabdl", "todo.json")))
	return reg
}
