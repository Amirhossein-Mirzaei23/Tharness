package session

import (
	"github.com/Amirhossein-Mirzaei23/Tharness/internal/ai"
)

// ResumeToMessages reconstructs an ai.Message history from transcript events,
// preserving order. Only "message" events contribute: user and plain
// assistant messages map directly; assistant tool_calls round-trip into
// ai.ToolCall; tool events become role "tool" messages carrying the recorded
// ToolResult keyed by ToolCallID. Non-message events (e.g. metadata) are
// skipped.
func ResumeToMessages(events []Event) []ai.Message {
	msgs := make([]ai.Message, 0, len(events))
	for _, ev := range events {
		if ev.Type != "message" {
			continue
		}
		switch ev.Role {
		case "user":
			msgs = append(msgs, ai.Message{Role: "user", Content: ev.Content})
		case "assistant":
			m := ai.Message{Role: "assistant", Content: ev.Content}
			for _, tc := range ev.ToolCalls {
				m.ToolCalls = append(m.ToolCalls, ai.ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: ai.FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				})
			}
			msgs = append(msgs, m)
		case "tool":
			content := ev.ToolResult
			if content == "" {
				content = ev.Content // legacy lines stored args in content
			}
			msgs = append(msgs, ai.Message{Role: "tool", Content: content, ToolCallID: ev.ToolCallID})
		}
	}
	return msgs
}
