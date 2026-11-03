package agent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Amirhossein-Mirzaei23/Tharness/internal/ai"
	"github.com/Amirhossein-Mirzaei23/Tharness/internal/tools"
)

// fakeProvider returns scripted Responses, one per Stream call, and records
// the requests it received.
type fakeProvider struct {
	mu                sync.Mutex
	responses         []ai.Response
	requests          []ai.Request
	blockUntilCtxDone bool // if true, Stream blocks until ctx is done

	deltas []string // content deltas emitted per call, for scripts
}

func (f *fakeProvider) Stream(ctx context.Context, req ai.Request, onDelta func(ai.Delta) error) (ai.Response, error) {
	f.mu.Lock()
	i := len(f.requests)
	f.requests = append(f.requests, req)
	f.mu.Unlock()

	if f.blockUntilCtxDone {
		<-ctx.Done()
		return ai.Response{}, ctx.Err()
	}
	if i >= len(f.responses) {
		if len(f.responses) == 0 {
			return ai.Response{}, errors.New("fakeProvider: no scripted response left")
		}
		// Repeat the last scripted response so MaxTurns tests can loop.
		resp := f.responses[len(f.responses)-1]
		if onDelta != nil && resp.Content != "" {
			if err := onDelta(ai.Delta{Content: resp.Content}); err != nil {
				return ai.Response{}, err
			}
		}
		return resp, nil
	}
	resp := f.responses[i]
	if onDelta != nil && resp.Content != "" {
		// Emit the whole content as one delta, like a single-chunk stream.
		if err := onDelta(ai.Delta{Content: resp.Content}); err != nil {
			return ai.Response{}, err
		}
	}
	return resp, nil
}

func (f *fakeProvider) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

func newFakeProvider(responses ...ai.Response) *fakeProvider {
	return &fakeProvider{responses: responses}
}

// fakeTool is a stub tool that records invocations and returns a canned
// result.
type fakeTool struct {
	name        string
	description string
	params      map[string]any
	result      string

	mu    sync.Mutex
	calls []string
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
	return f.result, nil
}

func (f *fakeTool) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// testLoop builds a Loop with an "echo" fakeTool registered.
func testLoop(p ai.Provider, maxTurns int) (*Loop, *fakeTool) {
	echo := &fakeTool{name: "echo", description: "echoes its input", result: "ECHOED"}
	reg := tools.NewRegistry()
	reg.Register(echo)
	return &Loop{Provider: p, Model: "test-model", Tools: reg, MaxTurns: maxTurns}, echo
}

func TestRunExecutesToolAndFeedsResultBack(t *testing.T) {
	p := newFakeProvider(
		ai.Response{
			Content: "let me echo that",
			ToolCalls: []ai.ToolCall{{
				ID:   "call_1",
				Type: "function",
				Function: ai.FunctionCall{
					Name:      "echo",
					Arguments: `{"text":"hello"}`,
				},
			}},
		},
		ai.Response{Content: "done"},
	)
	loop, echo := testLoop(p, 0) // 0 → default MaxTurns

	final, err := loop.Run(context.Background(), []ai.Message{{Role: "user", Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}

	if echo.callCount() != 1 {
		t.Fatalf("tool executed %d times, want 1", echo.callCount())
	}
	if got := strings.Join(echo.calls, ""); got != `{"text":"hello"}` {
		t.Errorf("tool args = %q, want {\"text\":\"hello\"}", got)
	}

	// History: [user, assistant(toolcall), tool(result), assistant(done)]
	if len(final) != 4 {
		t.Fatalf("history length = %d, want 4; msgs: %+v", len(final), final)
	}
	want := []struct{ role, content string }{
		{"user", "hi"},
		{"assistant", "let me echo that"},
		{"tool", "ECHOED"},
		{"assistant", "done"},
	}
	for i, w := range want {
		if final[i].Role != w.role {
			t.Errorf("final[%d].Role = %q, want %q", i, final[i].Role, w.role)
		}
		if final[i].Content != w.content {
			t.Errorf("final[%d].Content = %q, want %q", i, final[i].Content, w.content)
		}
	}
	if len(final[1].ToolCalls) != 1 {
		t.Fatalf("final[1].ToolCalls length = %d, want 1", len(final[1].ToolCalls))
	}
	if final[2].ToolCallID != "call_1" {
		t.Errorf("final[2].ToolCallID = %q, want call_1", final[2].ToolCallID)
	}

	if p.callCount() != 2 {
		t.Errorf("provider called %d times, want 2", p.callCount())
	}
	// The second request must include the tool result message.
	if p.requests[1].Messages[2].Role != "tool" || p.requests[1].Messages[2].Content != "ECHOED" {
		t.Errorf("second request did not include tool result: %+v", p.requests[1].Messages)
	}
	// Tools must be passed to the provider.
	if len(p.requests[0].Tools) != 1 || p.requests[0].Tools[0].Name != "echo" {
		t.Errorf("request tools = %+v, want [echo]", p.requests[0].Tools)
	}
}

func TestRunUnknownToolReturnsErrorResultToModel(t *testing.T) {
	p := newFakeProvider(
		ai.Response{
			ToolCalls: []ai.ToolCall{{
				ID:       "call_x",
				Type:     "function",
				Function: ai.FunctionCall{Name: "nope", Arguments: "{}"},
			}},
		},
		ai.Response{Content: "recovered"},
	)
	loop, _ := testLoop(p, 0)

	final, err := loop.Run(context.Background(), []ai.Message{{Role: "user", Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}

	if len(final) != 4 {
		t.Fatalf("history length = %d, want 4; msgs: %+v", len(final), final)
	}
	toolMsg := final[2]
	if toolMsg.Role != "tool" {
		t.Errorf("final[2].Role = %q, want tool", toolMsg.Role)
	}
	if !strings.Contains(toolMsg.Content, "unknown tool") {
		t.Errorf("tool result = %q, want it to contain %q", toolMsg.Content, "unknown tool")
	}
	if toolMsg.ToolCallID != "call_x" {
		t.Errorf("toolMsg.ToolCallID = %q, want call_x", toolMsg.ToolCallID)
	}
	if final[3].Content != "recovered" {
		t.Errorf("final turn content = %q, want recovered", final[3].Content)
	}
}

func TestRunMaxTurnsExceeded(t *testing.T) {
	toolCall := ai.ToolCall{ID: "c", Type: "function", Function: ai.FunctionCall{Name: "echo", Arguments: "{}"}}
	p := newFakeProvider(ai.Response{ToolCalls: []ai.ToolCall{toolCall}}) // always the same response
	loop, _ := testLoop(p, 2)

	_, err := loop.Run(context.Background(), []ai.Message{{Role: "user", Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("Run: err = nil, want MaxTurns exceeded error")
	}
	if !strings.Contains(err.Error(), "max turns") {
		t.Errorf("err = %v, want it to contain %q", err, "max turns")
	}
	if p.callCount() != 2 {
		t.Errorf("provider called %d times, want 2", p.callCount())
	}
}

func TestRunContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p := newFakeProvider(ai.Response{Content: "never reached"})
	p.blockUntilCtxDone = true
	loop, _ := testLoop(p, 0)

	errCh := make(chan error, 1)
	go func() {
		_, err := loop.Run(ctx, []ai.Message{{Role: "user", Content: "hi"}}, nil)
		errCh <- err
	}()
	cancel()
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Run err = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after ctx cancel")
	}
}

func TestRunEmitsEvents(t *testing.T) {
	p := newFakeProvider(
		ai.Response{
			Content: "let me echo that",
			ToolCalls: []ai.ToolCall{{
				ID:       "call_1",
				Type:     "function",
				Function: ai.FunctionCall{Name: "echo", Arguments: `{"text":"hello"}`},
			}},
		},
		ai.Response{Content: "done"},
	)
	loop, _ := testLoop(p, 0)

	var kinds []EventKind
	emit := func(e Event) { kinds = append(kinds, e.Kind) }

	_, err := loop.Run(context.Background(), []ai.Message{{Role: "user", Content: "hi"}}, emit)
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}

	want := []EventKind{EventTurnStart, EventDelta, EventToolCall, EventToolResult, EventTurnStart, EventDelta, EventTurnEnd}
	if len(kinds) != len(want) {
		t.Fatalf("event kinds = %v, want %v", kinds, want)
	}
	for i, w := range want {
		if kinds[i] != w {
			t.Errorf("kinds[%d] = %v, want %v (full: %v)", i, kinds[i], w, kinds)
		}
	}
}

func TestRunForwardsDeltas(t *testing.T) {
	p := newFakeProvider(ai.Response{Content: "done"})
	loop, _ := testLoop(p, 0)

	var deltas []string
	emit := func(e Event) {
		if e.Kind == EventDelta {
			deltas = append(deltas, e.Delta)
		}
	}
	_, err := loop.Run(context.Background(), []ai.Message{{Role: "user", Content: "hi"}}, emit)
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if len(deltas) == 0 {
		t.Error("no Delta events emitted, want at least one")
	}
	if got := strings.Join(deltas, ""); got != "done" {
		t.Errorf("deltas = %q, want %q", got, "done")
	}
}
