package session

import (
	"reflect"
	"testing"
	"time"

	"github.com/Amirhossein-Mirzaei23/Tharness/internal/ai"
)

func TestResumeToMessagesReconstructsHistory(t *testing.T) {
	ts := time.Date(2026, 11, 2, 15, 4, 5, 0, time.UTC).Format(time.RFC3339)
	events := []Event{
		{TS: ts, Type: "message", Role: "user", Content: "use echo"},
		{
			TS:   ts,
			Type: "message",
			Role: "assistant",
			ToolCalls: []ToolCall{{
				ID:   "call_1",
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{Name: "echo", Arguments: `{"q":"hi"}`},
			}},
		},
		{TS: ts, Type: "message", Role: "tool", Content: `{"q":"hi"}`, ToolCallID: "call_1", ToolName: "echo", ToolResult: "ECHOED"},
		{TS: ts, Type: "message", Role: "assistant", Content: "All done"},
		{TS: ts, Type: "metadata"}, // non-message events skipped
	}

	msgs := ResumeToMessages(events)
	want := []ai.Message{
		{Role: "user", Content: "use echo"},
		{
			Role: "assistant",
			ToolCalls: []ai.ToolCall{{
				ID:       "call_1",
				Type:     "function",
				Function: ai.FunctionCall{Name: "echo", Arguments: `{"q":"hi"}`},
			}},
		},
		{Role: "tool", Content: "ECHOED", ToolCallID: "call_1"},
		{Role: "assistant", Content: "All done"},
	}
	if len(msgs) != len(want) {
		t.Fatalf("ResumeToMessages returned %d messages, want %d: %+v", len(msgs), len(want), msgs)
	}
	for i, w := range want {
		if !reflect.DeepEqual(msgs[i], w) {
			t.Errorf("message %d = %+v, want %+v", i, msgs[i], w)
		}
	}
}
