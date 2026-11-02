package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// TodoItem is one entry in the persisted todo list.
type TodoItem struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Status  string `json:"status"` // "pending" | "in_progress" | "completed"
}

// Todo statuses.
const (
	TodoPending    = "pending"
	TodoInProgress = "in_progress"
	TodoCompleted  = "completed"
)

// TodoTool persists a todo list as a JSON file at a fixed path (one file per
// session; the orchestrator wires the session dir at construction).
//
// Policies (chosen, documented):
//   - Single in_progress invariant, ERROR policy: setting a second item to
//     in_progress returns an error describing the conflict; the caller must
//     complete or reset the current item first. This matches the
//     Hermes/omp todo semantics.
//   - Corrupt/unreadable file: ERROR on first access (no silent reset), so
//     data loss never happens without the caller noticing.
//   - Writes are atomic-ish: temp file in the same dir + rename.
type TodoTool struct {
	path string
}

// NewTodoTool returns a TodoTool persisting to filePath.
func NewTodoTool(filePath string) *TodoTool {
	return &TodoTool{path: filePath}
}

func (t *TodoTool) Name() string { return "todo" }

func (t *TodoTool) Description() string {
	return "Maintain a persistent task list. Actions: add (content), update (id, status, optional content), list, clear_completed. At most one item may be in_progress at a time."
}

func (t *TodoTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action":  map[string]any{"type": "string", "enum": []string{"add", "update", "list", "clear_completed"}, "description": "Operation to perform"},
			"content": map[string]any{"type": "string", "description": "Item text (add; optional replacement content for update)"},
			"id":      map[string]any{"type": "string", "description": "Item id (update)"},
			"status":  map[string]any{"type": "string", "enum": []string{"pending", "in_progress", "completed"}, "description": "New status (update)"},
		},
		"required": []string{"action"},
	}
}

func (t *TodoTool) Exec(_ context.Context, argsJSON string) (string, error) {
	var args struct {
		Action  string  `json:"action"`
		Content string  `json:"content"`
		ID      string  `json:"id"`
		Status  *string `json:"status"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("todo: parse args: %w", err)
	}
	switch args.Action {
	case "add":
		return t.add(args.Content)
	case "update":
		return t.update(args.ID, args.Status, args.Content)
	case "list":
		return t.list()
	case "clear_completed":
		return t.clearCompleted()
	default:
		return "", fmt.Errorf("todo: unknown action %q", args.Action)
	}
}

// list returns all items sorted by id (numeric-aware via the item sort).
func (t *TodoTool) list() (string, error) {
	items, err := t.load()
	if err != nil {
		return "", err
	}
	sort.Slice(items, func(a, b int) bool { return items[a].ID < items[b].ID })
	if len(items) == 0 {
		return "(no todo items)", nil
	}
	out := ""
	for _, it := range items {
		marker := "[ ]"
		switch it.Status {
		case TodoInProgress:
			marker = "[~]"
		case TodoCompleted:
			marker = "[x]"
		}
		out += fmt.Sprintf("%s %s: %s (%s)\n", marker, it.ID, it.Content, it.Status)
	}
	return out, nil
}

// clearCompleted drops every completed item (documented optional action).
func (t *TodoTool) clearCompleted() (string, error) {
	items, err := t.load()
	if err != nil {
		return "", err
	}
	kept := items[:0:0]
	removed := 0
	for _, it := range items {
		if it.Status == TodoCompleted {
			removed++
			continue
		}
		kept = append(kept, it)
	}
	if err := t.save(kept); err != nil {
		return "", err
	}
	return fmt.Sprintf("removed %d completed item(s)", removed), nil
}

func (t *TodoTool) update(id string, status *string, content string) (string, error) {
	if id == "" {
		return "", fmt.Errorf("todo: id is required for update")
	}
	if status == nil && content == "" {
		return "", fmt.Errorf("todo: update needs a status or content to change")
	}
	items, err := t.load()
	if err != nil {
		return "", err
	}
	for i, it := range items {
		if it.ID == id {
			if status != nil {
				if err := validTodoStatus(*status); err != nil {
					return "", err
				}
				// Single in_progress invariant (error policy): refuse to set a
				// second item in_progress while another one holds that status.
				if *status == TodoInProgress {
					for _, other := range items {
						if other.ID != id && other.Status == TodoInProgress {
							return "", fmt.Errorf("todo: item %s is already in_progress; complete or reset it before starting %s", other.ID, id)
						}
					}
				}
				items[i].Status = *status
			}
			if content != "" {
				items[i].Content = content
			}
			if err := t.save(items); err != nil {
				return "", err
			}
			return fmt.Sprintf("updated %s: status=%s", items[i].ID, items[i].Status), nil
		}
	}
	return "", fmt.Errorf("todo: no item with id %q", id)
}

func validTodoStatus(s string) error {
	switch s {
	case TodoPending, TodoInProgress, TodoCompleted:
		return nil
	default:
		return fmt.Errorf("todo: invalid status %q (want pending, in_progress, or completed)", s)
	}
}

func (t *TodoTool) add(content string) (string, error) {
	if content == "" {
		return "", fmt.Errorf("todo: content is required for add")
	}
	items, err := t.load()
	if err != nil {
		return "", err
	}
	item := TodoItem{ID: nextTodoID(items), Content: content, Status: TodoPending}
	items = append(items, item)
	if err := t.save(items); err != nil {
		return "", err
	}
	return fmt.Sprintf("added %s: %s", item.ID, item.Content), nil
}

// load reads and parses the todo file. A missing file is an empty list;
// a corrupt or unreadable file is an error (documented policy: never reset
// silently).
func (t *TodoTool) load() ([]TodoItem, error) {
	data, err := os.ReadFile(t.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("todo: read %s: %w", t.path, err)
	}
	var items []TodoItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("todo: corrupt todo file %s: %w", t.path, err)
	}
	return items, nil
}

// save writes items atomically-ish: temp file in the same directory, then
// rename over the target.
func (t *TodoTool) save(items []TodoItem) error {
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("todo: encode: %w", err)
	}
	dir := filepath.Dir(t.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("todo: create dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".todo-*")
	if err != nil {
		return fmt.Errorf("todo: temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("todo: write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("todo: close temp: %w", err)
	}
	if err := os.Rename(tmpName, t.path); err != nil {
		return fmt.Errorf("todo: rename: %w", err)
	}
	return nil
}

// nextTodoID returns the smallest unused numeric id as a string, e.g. "1",
// "2", ... so ids stay stable and sortable across add/clear cycles.
func nextTodoID(items []TodoItem) string {
	used := make(map[int]bool, len(items))
	max := 0
	for _, it := range items {
		var n int
		if _, err := fmt.Sscanf(it.ID, "%d", &n); err == nil {
			used[n] = true
			if n > max {
				max = n
			}
		}
	}
	for n := 1; n <= max+1; n++ {
		if !used[n] {
			return fmt.Sprintf("%d", n)
		}
	}
	return fmt.Sprintf("%d", max+1)
}
