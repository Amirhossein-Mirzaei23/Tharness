package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Amirhossein-Mirzaei23/Tharness/internal/session"
)

// TestRootResumeFlagSeedsPriorMessages verifies the root --resume flag is
// wired through cobra into RunOptions.Resume.
func TestRootResumeFlagSeedsPriorMessages(t *testing.T) {
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
		_, _ = w.Write([]byte(textSSE))
	}))
	defer srv.Close()

	dataDir := t.TempDir()
	t.Setenv("THARNESS_BASE_URL", srv.URL)
	t.Setenv("THARNESS_API_KEY", "test-key")
	t.Setenv("THARNESS_DATA_DIR", dataDir)

	store, err := session.Open(dataDir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	id, path, err := store.New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	ts := time.Now().UTC().Format(time.RFC3339)
	for _, ev := range []session.Event{
		{TS: ts, Type: "message", Role: "user", Content: "prior question"},
		{TS: ts, Type: "message", Role: "assistant", Content: "prior answer"},
	} {
		if err := store.Append(path, ev); err != nil {
			t.Fatalf("Append() error: %v", err)
		}
	}

	root := NewRootCommand()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetArgs([]string{"-p", "new question", "--resume", id})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	got := lastReq.Messages
	if len(got) != 3 {
		t.Fatalf("provider got %d messages, want 3: %+v", len(got), got)
	}
	if got[0].Content != "prior question" || got[1].Content != "prior answer" || got[2].Content != "new question" {
		t.Errorf("messages = %+v, want prior question, prior answer, new question", got)
	}
}
