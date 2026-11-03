package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Amirhossein-Mirzaei23/Tharness/internal/session"
)

// TestRunResumeSeedsPriorMessages verifies that Run with Resume loads the
// stored session's messages before the new user prompt: the fake provider
// must receive prior user/assistant messages followed by the fresh prompt.
func TestRunResumeSeedsPriorMessages(t *testing.T) {
	var lastReq struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&lastReq); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, textSSE) //nolint:errcheck // test fake
	}))
	defer srv.Close()

	dataDir := t.TempDir()
	t.Setenv("THARNESS_BASE_URL", srv.URL)
	t.Setenv("THARNESS_API_KEY", "test-key")
	t.Setenv("THARNESS_DATA_DIR", dataDir)

	// Pre-seed a prior session via the store.
	store, err := session.Open(dataDir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	id, path, err := store.New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	ts := time.Now().UTC().Format(time.RFC3339)
	seeds := []session.Event{
		{TS: ts, Type: "message", Role: "user", Content: "prior question"},
		{TS: ts, Type: "message", Role: "assistant", Content: "prior answer"},
	}
	for _, ev := range seeds {
		if err := store.Append(path, ev); err != nil {
			t.Fatalf("Append() error: %v", err)
		}
	}

	opts := RunOptions{Prompt: "new question", Resume: id}
	stdout := &bytes.Buffer{}
	if err := opts.Run(t.Context(), stdout); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	got := lastReq.Messages
	if len(got) != 3 {
		t.Fatalf("provider got %d messages, want 3: %+v", len(got), got)
	}
	if got[0].Role != "user" || got[0].Content != "prior question" {
		t.Errorf("msg[0] = %+v, want user/prior question", got[0])
	}
	if got[1].Role != "assistant" || got[1].Content != "prior answer" {
		t.Errorf("msg[1] = %+v, want assistant/prior answer", got[1])
	}
	if got[2].Role != "user" || got[2].Content != "new question" {
		t.Errorf("msg[2] = %+v, want user/new question", got[2])
	}
	if got := strings.TrimSpace(stdout.String()); got != "Hello world" {
		t.Errorf("stdout = %q, want %q", got, "Hello world")
	}
}

// TestRunResumeUnknownIDFails verifies a clear error for a missing session id.
func TestRunResumeUnknownIDFails(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("THARNESS_DATA_DIR", dataDir)
	t.Setenv("THARNESS_BASE_URL", "http://127.0.0.1:1") // must not be reached
	t.Setenv("THARNESS_API_KEY", "test-key")

	opts := RunOptions{Prompt: "x", Resume: "does-not-exist"}
	if err := opts.Run(t.Context(), &bytes.Buffer{}); err == nil {
		t.Fatal("Run() with unknown resume id: got nil error, want failure")
	}
}
