package tui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// chatSSE is a minimal OpenAI-compatible text-only SSE stream.
const chatSSE = "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n" +
	"data: [DONE]\n\n"

// RED: ChatRunner (the real AgentRunner) streams agent events and closes the
// channel; the transcript is appended to the sessions dir.
func TestChatRunnerStreamsAndCloses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(chatSSE))
	}))
	defer srv.Close()

	dataDir := t.TempDir()
	t.Setenv("THARNESS_BASE_URL", srv.URL)
	t.Setenv("THARNESS_API_KEY", "test-key")
	t.Setenv("THARNESS_DATA_DIR", dataDir)

	runner, err := NewChatRunner("")
	if err != nil {
		t.Fatalf("NewChatRunner() error: %v", err)
	}

	events, cancel := runner.Run(context.Background(), "hello")
	defer cancel()

	var sawDelta, sawTurnEnd bool
	deadline := time.After(5 * time.Second)
	for done := false; !done; {
		select {
		case ev, ok := <-events:
			if !ok {
				done = true
				break
			}
			switch ev.Kind {
			case "delta":
				sawDelta = true
			case "turn_end":
				sawTurnEnd = true
			}
		case <-deadline:
			t.Fatal("timeout waiting for event channel close")
		}
	}
	if !sawDelta || !sawTurnEnd {
		t.Errorf("sawDelta=%v sawTurnEnd=%v, want both", sawDelta, sawTurnEnd)
	}

	// Transcript written: exactly one .jsonl file mentioning the prompt.
	matches, err := filepath.Glob(filepath.Join(dataDir, "sessions", "*.jsonl"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("session files = %v err=%v, want exactly 1", matches, err)
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	if !strings.Contains(string(data), "hello") {
		t.Errorf("transcript missing user prompt")
	}
	// First line must be valid JSON (store format).
	line := strings.SplitN(strings.TrimSpace(string(data)), "\n", 2)[0]
	var ev map[string]any
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		t.Errorf("first transcript line not JSON: %v", err)
	}
}
