package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// OpenAI is a Provider for OpenAI-compatible chat-completions endpoints
// (OpenAI, zai, OpenRouter, Ollama, vLLM, ...).
type OpenAI struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

type wireRequest struct {
	Model         string         `json:"model"`
	Messages      []Message      `json:"messages"`
	Tools         []wireTool     `json:"tools,omitempty"`
	MaxTokens     int            `json:"max_tokens,omitempty"`
	Temperature   float64        `json:"temperature,omitempty"`
	Stream        bool           `json:"stream"`
	StreamOptions *streamOptions `json:"stream_options,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type wireTool struct {
	Type     string       `json:"type"`
	Function wireFunction `json:"function"`
}

type wireFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type wireChunk struct {
	Choices []struct {
		Index        int    `json:"index"`
		Delta        delta  `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *usage `json:"usage"`
}

type delta struct {
	Content   string    `json:"content"`
	ToolCalls []tcDelta `json:"tool_calls"`
}

type tcDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// Stream POSTs a streaming chat-completions request and feeds content deltas
// to onDelta as they arrive. Tool-call fragments are accumulated by index
// server-side and returned in the final Response.
func (o *OpenAI) Stream(ctx context.Context, req Request, onDelta func(Delta) error) (Response, error) {
	var resp Response

	body := wireRequest{
		Model:         req.Model,
		Messages:      req.Messages,
		MaxTokens:     req.MaxTokens,
		Temperature:   req.Temperature,
		Stream:        true,
		StreamOptions: &streamOptions{IncludeUsage: true},
	}
	for _, t := range req.Tools {
		body.Tools = append(body.Tools, wireTool{
			Type: "function",
			Function: wireFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return resp, fmt.Errorf("ai: encode request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimSuffix(o.BaseURL, "/")+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return resp, fmt.Errorf("ai: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if o.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+o.APIKey)
	}

	client := o.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return resp, fmt.Errorf("ai: stream request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode < 200 || httpResp.StatusCode > 299 {
		var buf bytes.Buffer
		buf.ReadFrom(httpResp.Body) //nolint:errcheck // best-effort body capture
		return resp, &APIError{StatusCode: httpResp.StatusCode, Body: buf.String()}
	}

	acc := newToolCallAccumulator()
	scanner := bufio.NewScanner(httpResp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk wireChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return resp, fmt.Errorf("ai: decode chunk: %w", err)
		}
		if chunk.Usage != nil {
			resp.Usage = Usage{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
			}
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				resp.Content += choice.Delta.Content
				if onDelta != nil {
					if err := onDelta(Delta{Content: choice.Delta.Content}); err != nil {
						return resp, err
					}
				}
			}
			if len(choice.Delta.ToolCalls) > 0 {
				acc.add(choice.Delta.ToolCalls)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return resp, fmt.Errorf("ai: read stream: %w", err)
	}
	resp.ToolCalls = acc.finish()
	return resp, nil
}

// toolCallAccumulator merges streamed tool-call fragments by index.
type toolCallAccumulator struct {
	byIndex map[int]*ToolCall
	order   []int
}

func newToolCallAccumulator() *toolCallAccumulator {
	return &toolCallAccumulator{byIndex: map[int]*ToolCall{}}
}

func (a *toolCallAccumulator) add(frags []tcDelta) {
	for _, f := range frags {
		tc, ok := a.byIndex[f.Index]
		if !ok {
			tc = &ToolCall{ID: f.ID, Type: f.Type, Function: FunctionCall{Name: f.Function.Name}}
			a.byIndex[f.Index] = tc
			a.order = append(a.order, f.Index)
			continue
		}
		if tc.ID == "" {
			tc.ID = f.ID
		}
		if tc.Type == "" {
			tc.Type = f.Type
		}
		if tc.Function.Name == "" {
			tc.Function.Name = f.Function.Name
		}
	}
	a.appendArgs(frags)
}

func (a *toolCallAccumulator) appendArgs(frags []tcDelta) {
	for _, f := range frags {
		if tc, ok := a.byIndex[f.Index]; ok {
			tc.Function.Arguments += f.Function.Arguments
		}
	}
}

func (a *toolCallAccumulator) finish() []ToolCall {
	out := make([]ToolCall, 0, len(a.order))
	for _, i := range a.order {
		out = append(out, *a.byIndex[i])
	}
	return out
}
