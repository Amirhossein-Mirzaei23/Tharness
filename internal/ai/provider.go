package ai

import (
	"context"
	"fmt"
)

// Message is a chat message in OpenAI chat-completions form.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolCall identifies a function invocation requested by the model.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall is the function name plus its JSON-encoded arguments.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Tool is a function the model may call.
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any
}

// Delta is an incremental piece of a streamed assistant turn.
type Delta struct {
	Content        string
	ToolCallDeltas []ToolCall
}

// Request is a chat-completions request.
type Request struct {
	Model       string
	Messages    []Message
	Tools       []Tool
	MaxTokens   int
	Temperature float64
}

// Response is the accumulated result of a streamed turn.
type Response struct {
	Content   string
	ToolCalls []ToolCall
	Usage     Usage
}

// Usage reports token consumption for the turn.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
}

// APIError is a non-2xx response from the provider.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("ai: provider returned status %d: %s", e.StatusCode, e.Body)
}

// Provider streams chat completions from an LLM backend. onDelta is called
// once per streamed content delta; returning an error from onDelta aborts
// the stream.
type Provider interface {
	Stream(ctx context.Context, req Request, onDelta func(Delta) error) (Response, error)
}
