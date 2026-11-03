package agent

import "github.com/Amirhossein-Mirzaei23/Tharness/internal/ai"

// EventKind classifies an agent-loop event.
type EventKind string

const (
	EventTurnStart  EventKind = "turn_start"
	EventDelta      EventKind = "delta"
	EventToolCall   EventKind = "tool_call"
	EventToolResult EventKind = "tool_result"
	EventTurnEnd    EventKind = "turn_end"
)

// Event is a notification emitted by the agent loop as it progresses.
// Which fields are populated depends on Kind:
//
//	EventTurnStart  — Turn index is not tracked; no payload fields set.
//	EventDelta      — Delta holds a content chunk.
//	EventToolCall   — ToolCall points to the requested call.
//	EventToolResult — ToolName and ToolResult are set.
//	EventTurnEnd    — no payload fields set.
type Event struct {
	Kind       EventKind
	Delta      string
	ToolCall   *ai.ToolCall
	ToolName   string
	ToolResult string
	Err        error
}
