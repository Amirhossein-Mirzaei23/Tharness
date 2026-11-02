package session

import (
	"reflect"
	"testing"
	"time"
)

// mustStore opens a Store in a fresh temp dir.
func mustStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	return s
}

func TestAppendLoadRoundTrip(t *testing.T) {
	s := mustStore(t)
	id, path, err := s.New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ts := time.Date(2026, 11, 2, 15, 4, 5, 0, time.UTC)
	ev1 := Event{TS: ts.Format(time.RFC3339), Type: "message", Role: "user", Content: "hello"}
	ev2 := Event{TS: ts.Format(time.RFC3339), Type: "message", Role: "assistant", Content: "hi there"}
	ev3 := Event{
		TS:         ts.Format(time.RFC3339),
		Type:       "message",
		Role:       "tool",
		Content:    `{"q":"hi"}`,
		ToolCallID: "call_1",
		ToolName:   "echo",
		ToolResult: "ECHOED",
	}
	for _, ev := range []Event{ev1, ev2, ev3} {
		if err := s.Append(path, ev); err != nil {
			t.Fatalf("Append(%+v) error: %v", ev, err)
		}
	}

	got, warnings, err := s.Load(id)
	if err != nil {
		t.Fatalf("Load(%q) error: %v", id, err)
	}
	if len(warnings) != 0 {
		t.Fatalf("Load returned warnings: %v", warnings)
	}
	if len(got) != 3 {
		t.Fatalf("Load returned %d events, want 3: %+v", len(got), got)
	}
	for i, want := range []Event{ev1, ev2, ev3} {
		if !reflect.DeepEqual(got[i], want) {
			t.Errorf("event %d = %+v, want %+v", i, got[i], want)
		}
	}
}
