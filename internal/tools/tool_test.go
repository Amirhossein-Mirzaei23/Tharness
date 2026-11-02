package tools

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// fakeTool is a stub tool that records invocations and returns a canned
// result. Used across the tools and agent tests.
type fakeTool struct {
	name        string
	description string
	params      map[string]any
	result      string
	err         error

	mu      sync.Mutex
	calls   []string // argsJSON of each Exec invocation
	execErr error
}

func (f *fakeTool) Name() string        { return f.name }
func (f *fakeTool) Description() string { return f.description }
func (f *fakeTool) Parameters() map[string]any {
	return f.params
}
func (f *fakeTool) Exec(ctx context.Context, argsJSON string) (string, error) {
	f.mu.Lock()
	f.calls = append(f.calls, argsJSON)
	f.mu.Unlock()
	if f.execErr != nil {
		return "", f.execErr
	}
	return f.result, nil
}

func (f *fakeTool) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func TestRegistryRegisterAndGet(t *testing.T) {
	echo := &fakeTool{name: "echo", description: "echoes input", result: "hi"}
	reg := NewRegistry()
	reg.Register(echo)

	got, ok := reg.Get("echo")
	if !ok {
		t.Fatal("Get(echo) returned ok=false, want true")
	}
	if got.Name() != "echo" {
		t.Errorf("Get(echo).Name() = %q, want %q", got.Name(), "echo")
	}
	if _, ok := reg.Get("missing"); ok {
		t.Error("Get(missing) returned ok=true, want false")
	}
}

func TestRegistryNames(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&fakeTool{name: "alpha"})
	reg.Register(&fakeTool{name: "beta"})

	got := reg.Names()
	if len(got) != 2 {
		t.Fatalf("Names() = %v, want 2 entries", got)
	}
	joined := strings.Join(got, ",")
	for _, want := range []string{"alpha", "beta"} {
		if !strings.Contains(joined, want) {
			t.Errorf("Names() = %v, missing %q", got, want)
		}
	}
}

func TestRegistryUnknownToolError(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Exec(context.Background(), "missing", "{}")
	if err == nil {
		t.Fatal("Exec on unknown tool: err = nil, want error")
	}
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("Exec on unknown tool: err = %v, want it to contain %q", err, "unknown tool")
	}
}

func TestRegistryExecInvokesTool(t *testing.T) {
	echo := &fakeTool{name: "echo", result: "hi"}
	reg := NewRegistry()
	reg.Register(echo)

	res, err := reg.Exec(context.Background(), "echo", `{"x":1}`)
	if err != nil {
		t.Fatalf("Exec: unexpected error: %v", err)
	}
	if res != "hi" {
		t.Errorf("Exec = %q, want %q", res, "hi")
	}
	if echo.callCount() != 1 {
		t.Errorf("tool executed %d times, want 1", echo.callCount())
	}
	if fmt.Sprint(echo.calls) != fmt.Sprint([]string{`{"x":1}`}) {
		t.Errorf("tool args = %v, want [{\"x\":1}]", echo.calls)
	}
}
