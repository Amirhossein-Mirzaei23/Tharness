package session

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// Event is one JSONL record in a session transcript. Exactly one event per
// line; unknown JSON fields are ignored on load.
type Event struct {
	// TS is the event time, RFC3339.
	TS string `json:"ts"`
	// Type distinguishes record kinds: "message", "metadata", ...
	Type string `json:"type"`
	// Role is the chat role for message events: user/assistant/tool.
	Role string `json:"role,omitempty"`
	// Content carries the message text; for tool messages it holds the tool
	// call arguments as recorded by the CLI transcript.
	Content string `json:"content,omitempty"`
	// ToolCalls holds assistant tool-call requests (round-tripped for resume).
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// ToolCallID is the assistant call id a tool result responds to.
	ToolCallID string `json:"tool_call_id,omitempty"`
	// ToolName is the invoked tool's name for tool-result events.
	ToolName string `json:"tool_name,omitempty"`
	// ToolResult is the tool's output text for tool-result events.
	ToolResult string `json:"tool_result,omitempty"`
}

// ToolCall mirrors an assistant function call in transcript form.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Append encodes ev as one JSONL line and appends it to the session file at
// path. It is safe for concurrent use.
func (s *Store) Append(path string, ev Event) error {
	data, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("session: encode event: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("session: open transcript: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only closer
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("session: append event: %w", err)
	}
	return nil
}

// Load reads all events from the session with the given id. A trailing empty
// line is tolerated; corrupt (non-JSON) lines are skipped and reported via the
// returned warnings slice — loading is never fatal for corrupt lines.
func (s *Store) Load(id string) ([]Event, []string, error) {
	f, err := os.Open(s.Path(id))
	if err != nil {
		return nil, nil, fmt.Errorf("session: open transcript: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only closer

	var (
		evs      []Event
		warnings []string
	)
	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		raw := sc.Bytes()
		if len(bytes.TrimSpace(raw)) == 0 {
			continue // tolerate blank/trailing lines
		}
		var ev Event
		if err := json.Unmarshal(raw, &ev); err != nil {
			warnings = append(warnings, fmt.Sprintf("line %d: skipped corrupt line: %v", line, err))
			continue
		}
		evs = append(evs, ev)
	}
	if err := sc.Err(); err != nil {
		return evs, warnings, fmt.Errorf("session: scan transcript: %w", err)
	}
	return evs, warnings, nil
}
