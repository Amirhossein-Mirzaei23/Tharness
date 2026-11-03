package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Amirhossein-Mirzaei23/Tharness/internal/tools"
)

// textSSE is the OpenAI-compatible SSE stream served by the fake provider for
// the text-only test (same chunk format as internal/ai/openai_test.go).
const textSSE = "data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hello\"}}]}\n\n" +
	"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\" world\"}}]}\n\n" +
	"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":11,\"completion_tokens\":3}}\n\n" +
	"data: [DONE]\n\n"

// sseServer returns an httptest server that replies with the given raw SSE
// body and records the last Authorization header it saw.
func sseServer(t *testing.T, sse string, auth *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*auth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sse) //nolint:errcheck // test fake
	}))
	t.Cleanup(srv.Close)
	return srv
}

// readTranscript reads a JSONL transcript into a slice of generic objects.
func readTranscript(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	var lines []map[string]any
	for _, ln := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(ln), &m); err != nil {
			t.Fatalf("transcript line %q is not JSON: %v", ln, err)
		}
		lines = append(lines, m)
	}
	return lines
}

func TestRunOneShotPrintsFinalTextAndWritesTranscript(t *testing.T) {
	var auth string
	srv := sseServer(t, textSSE, &auth)

	dataDir := t.TempDir()
	t.Setenv("THARNESS_BASE_URL", srv.URL)
	t.Setenv("THARNESS_API_KEY", "test-key")
	t.Setenv("THARNESS_DATA_DIR", dataDir)

	opts := RunOptions{Prompt: "say hello"}
	stdout := &bytes.Buffer{}
	err := opts.Run(t.Context(), stdout)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if got := strings.TrimSpace(stdout.String()); got != "Hello world" {
		t.Fatalf("stdout = %q, want %q", got, "Hello world")
	}
	if auth != "Bearer test-key" {
		t.Fatalf("Authorization = %q, want %q", auth, "Bearer test-key")
	}

	entries, err := os.ReadDir(filepath.Join(dataDir, "sessions"))
	if err != nil {
		t.Fatalf("sessions dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d session files, want 1", len(entries))
	}
	lines := readTranscript(t, filepath.Join(dataDir, "sessions", entries[0].Name()))
	if len(lines) < 2 {
		t.Fatalf("transcript has %d lines, want >= 2: %v", len(lines), lines)
	}
	if lines[0]["role"] != "user" {
		t.Fatalf("first role = %v, want user", lines[0]["role"])
	}
	if lines[0]["content"] != "say hello" {
		t.Fatalf("first content = %v, want %q", lines[0]["content"], "say hello")
	}
	last := lines[len(lines)-1]
	if last["role"] != "assistant" || last["content"] != "Hello world" {
		t.Fatalf("last line = %v, want assistant/Hello world", last)
	}
	for i, ln := range lines {
		if _, ok := ln["ts"]; !ok {
			t.Errorf("line %d missing ts field", i)
		}
	}
}

// seqServer returns a server that serves each SSE body in order per request.
func seqServer(t *testing.T, bodies ...string) *httptest.Server {
	t.Helper()
	i := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := bodies[min(i, len(bodies)-1)]
		i++
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, body) //nolint:errcheck // test fake
	}))
	t.Cleanup(srv.Close)
	return srv
}

// fakeTool is a cli-package stub tool that records invocations.
type fakeTool struct {
	name   string
	result string

	calls []string
}

func (f *fakeTool) Name() string               { return f.name }
func (f *fakeTool) Description() string        { return "test tool " + f.name }
func (f *fakeTool) Parameters() map[string]any { return map[string]any{"type": "object"} }
func (f *fakeTool) Exec(_ context.Context, argsJSON string) (string, error) {
	f.calls = append(f.calls, argsJSON)
	return f.result, nil
}

func (f *fakeTool) callCount() int { return len(f.calls) }

// toolCallSSE serves the turn-1 stream: an assistant turn requesting one tool
// call (same chunk format as internal/ai/openai_test.go).
const toolCallSSE = "data: {\"id\":\"c2\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"echo\",\"arguments\":\"{\\\"q\\\"\"}}]}}]}\n\n" +
	"data: {\"id\":\"c2\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\":\\\"hi\\\"}\"}}]}}]}\n\n" +
	"data: {\"id\":\"c2\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n" +
	"data: [DONE]\n\n"

// finalSSE is the turn-2 stream: a plain final text answer.
const finalSSE = "data: {\"id\":\"c3\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"All done\"}}]}\n\n" +
	"data: {\"id\":\"c3\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
	"data: [DONE]\n\n"

func TestRunOneShotExecutesToolCallAndRecordsResult(t *testing.T) {
	srv := seqServer(t, toolCallSSE, finalSSE)

	dataDir := t.TempDir()
	t.Setenv("THARNESS_BASE_URL", srv.URL)
	t.Setenv("THARNESS_API_KEY", "test-key")
	t.Setenv("THARNESS_DATA_DIR", dataDir)

	echo := &fakeTool{name: "echo", result: "ECHOED"}
	registry := tools.NewRegistry()
	registry.Register(echo)

	opts := RunOptions{Prompt: "use echo", Tools: registry}
	stdout := &bytes.Buffer{}
	if err := opts.Run(t.Context(), stdout); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if echo.callCount() != 1 {
		t.Fatalf("tool executed %d times, want 1", echo.callCount())
	}
	if got := strings.Join(echo.calls, ""); !strings.Contains(got, `"q"`) {
		t.Fatalf("tool args = %q, want JSON containing %q", got, `"q"`)
	}
	if got := strings.TrimSpace(stdout.String()); got != "All done" {
		t.Fatalf("stdout = %q, want %q", got, "All done")
	}

	entries, err := os.ReadDir(filepath.Join(dataDir, "sessions"))
	if err != nil {
		t.Fatalf("sessions dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d session files, want 1", len(entries))
	}
	lines := readTranscript(t, filepath.Join(dataDir, "sessions", entries[0].Name()))

	var sawToolResult bool
	for _, ln := range lines {
		if ln["role"] == "tool" {
			sawToolResult = true
			if ln["tool_name"] != "echo" {
				t.Errorf("tool line tool_name = %v, want echo", ln["tool_name"])
			}
			if ln["tool_result"] != "ECHOED" {
				t.Errorf("tool line tool_result = %v, want ECHOED", ln["tool_result"])
			}
		}
	}
	if !sawToolResult {
		t.Fatalf("transcript has no tool line: %v", lines)
	}
}
