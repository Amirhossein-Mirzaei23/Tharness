package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTodoTool returns a TodoTool backed by a file inside a fresh temp dir.
func newTodoTool(t *testing.T) (*TodoTool, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.json")
	return NewTodoTool(path), path
}

func TestTodoUpdateSetsInProgress(t *testing.T) {
	tool, path := newTodoTool(t)
	if _, err := tool.Exec(context.Background(), `{"action":"add","content":"task one"}`); err != nil {
		t.Fatalf("add: %v", err)
	}
	var items []TodoItem
	data, _ := os.ReadFile(path)
	json.Unmarshal(data, &items)
	id := items[0].ID

	if _, err := tool.Exec(context.Background(), fmt.Sprintf(`{"action":"update","id":%q,"status":"in_progress"}`, id)); err != nil {
		t.Fatalf("update to in_progress: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatal(err)
	}
	if items[0].Status != "in_progress" {
		t.Fatalf("status = %q, want in_progress", items[0].Status)
	}
}

func TestTodoCompleteFreesInProgressSlot(t *testing.T) {
	tool, _ := newTodoTool(t)
	tool.Exec(context.Background(), `{"action":"add","content":"first"}`)
	tool.Exec(context.Background(), `{"action":"add","content":"second"}`)
	tool.Exec(context.Background(), `{"action":"update","id":"1","status":"in_progress"}`)
	if _, err := tool.Exec(context.Background(), `{"action":"update","id":"2","status":"in_progress"}`); err == nil {
		t.Fatal("precondition: second in_progress must conflict")
	}
	if _, err := tool.Exec(context.Background(), `{"action":"update","id":"1","status":"completed"}`); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if _, err := tool.Exec(context.Background(), `{"action":"update","id":"2","status":"in_progress"}`); err != nil {
		t.Fatalf("after completing, another item may become in_progress: %v", err)
	}
}

func TestTodoUpdateUnknownIDErrors(t *testing.T) {
	tool, _ := newTodoTool(t)
	tool.Exec(context.Background(), `{"action":"add","content":"real"}`)

	_, err := tool.Exec(context.Background(), `{"action":"update","id":"99","status":"completed"}`)
	if err == nil {
		t.Fatal("update with unknown id must return an error")
	}
	if !strings.Contains(err.Error(), `no item with id "99"`) {
		t.Fatalf("error should name the unknown id, got: %v", err)
	}
}

func TestTodoAddEmptyContentErrors(t *testing.T) {
	tool, _ := newTodoTool(t)

	_, err := tool.Exec(context.Background(), `{"action":"add","content":""}`)
	if err == nil {
		t.Fatal("add with empty content must return an error")
	}
	if !strings.Contains(err.Error(), "content is required") {
		t.Fatalf("error should mention content requirement, got: %v", err)
	}
}

func TestTodoListSortedByID(t *testing.T) {
	tool, _ := newTodoTool(t)
	// Add in non-numeric order; numeric ids keep list order stable and small.
	for _, c := range []string{"alpha", "beta", "gamma"} {
		tool.Exec(context.Background(), fmt.Sprintf(`{"action":"add","content":%q}`, c))
	}

	out, err := tool.Exec(context.Background(), `{"action":"list"}`)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var got []string
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		got = append(got, strings.TrimSpace(line))
	}
	if len(got) != 3 {
		t.Fatalf("list should show 3 items, got %d: %q", len(got), out)
	}
	for i, want := range []string{"1", "2", "3"} {
		if !strings.HasPrefix(got[i], "[ ] "+want+":") {
			t.Fatalf("line %d = %q, want item %s in id order", i, got[i], want)
		}
	}
}

func TestTodoCorruptFileErrors(t *testing.T) {
	tool, path := newTodoTool(t)
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Documented policy: corrupt file is an ERROR on first access, never a
	// silent reset (no data loss without the caller noticing).
	for _, args := range []string{
		`{"action":"list"}`,
		`{"action":"add","content":"x"}`,
		`{"action":"update","id":"1","status":"completed"}`,
		`{"action":"clear_completed"}`,
	} {
		_, err := tool.Exec(context.Background(), args)
		if err == nil {
			t.Fatalf("corrupt file must error on access, action %s succeeded", args)
		}
		if !strings.Contains(err.Error(), "corrupt") {
			t.Fatalf("error should mention corruption, got: %v", err)
		}
	}
	// The corrupt file must be left untouched (no reset).
	data, _ := os.ReadFile(path)
	if string(data) != "{not json" {
		t.Fatalf("corrupt file was modified: %q", data)
	}
}

func TestTodoSecondInProgressConflicts(t *testing.T) {
	tool, _ := newTodoTool(t)
	add := func(content string) string {
		out, err := tool.Exec(context.Background(), fmt.Sprintf(`{"action":"add","content":%q}`, content))
		if err != nil {
			t.Fatalf("add %q: %v", content, err)
		}
		return out
	}
	_ = add("first")
	second := add("second")
	id2 := strings.TrimSpace(strings.TrimPrefix(second, "added "))
	// extract id: output is "added N: content"
	id2 = strings.SplitN(id2, ":", 2)[0]

	if _, err := tool.Exec(context.Background(), `{"action":"update","id":"1","status":"in_progress"}`); err != nil {
		t.Fatalf("first in_progress should succeed: %v", err)
	}
	_, err := tool.Exec(context.Background(), fmt.Sprintf(`{"action":"update","id":%q,"status":"in_progress"}`, id2))
	if err == nil {
		t.Fatal("setting a second in_progress must return an error")
	}
	if !strings.Contains(err.Error(), "in_progress") {
		t.Fatalf("error should describe the conflict, got: %v", err)
	}
}

func TestNewCoreRegistryIncludesTodo(t *testing.T) {
	reg := NewCoreRegistry(t.TempDir())
	if _, ok := reg.Get("todo"); !ok {
		t.Fatal("NewCoreRegistry should register todo")
	}
}

func TestTodoAddCreatesPendingItemInFile(t *testing.T) {
	tool, path := newTodoTool(t)

	out, err := tool.Exec(context.Background(), `{"action":"add","content":"write tests"}`)
	if err != nil {
		t.Fatalf("Exec add: %v", err)
	}
	if !strings.Contains(out, "write tests") {
		t.Fatalf("confirmation should mention content, got: %s", out)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("todo.json not created: %v", err)
	}
	var items []TodoItem
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatalf("todo.json is not valid JSON: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Content != "write tests" {
		t.Fatalf("content = %q, want %q", items[0].Content, "write tests")
	}
	if items[0].Status != "pending" {
		t.Fatalf("status = %q, want %q", items[0].Status, "pending")
	}
	if items[0].ID == "" {
		t.Fatal("item must have a generated non-empty id")
	}
}
