package ai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOpenAIStreamYieldsDeltasAndFinalUsage(t *testing.T) {
	sse := "data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hello\"}}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\" world\"}}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":11,\"completion_tokens\":3}}\n\n" +
		"data: [DONE]\n\n"

	var gotRequest struct {
		Body      string
		Auth      string
		StreamOpt string
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequest.Body = r.URL.Path
		gotRequest.Auth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sse)
	}))
	defer srv.Close()

	p := &OpenAI{BaseURL: srv.URL, APIKey: "test-key", HTTPClient: srv.Client()}

	var deltas []string
	resp, err := p.Stream(context.Background(), Request{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: "hi"}},
	}, func(d Delta) error {
		deltas = append(deltas, d.Content)
		return nil
	})
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	if len(deltas) != 2 {
		t.Fatalf("got %d deltas (%q), want 2", len(deltas), deltas)
	}
	if deltas[0] != "Hello" || deltas[1] != " world" {
		t.Fatalf("deltas = %q, want [Hello, \" world\"]", deltas)
	}
	if resp.Content != "Hello world" {
		t.Fatalf("resp.Content = %q, want %q", resp.Content, "Hello world")
	}
	if resp.Usage.PromptTokens != 11 || resp.Usage.CompletionTokens != 3 {
		t.Fatalf("resp.Usage = %+v, want prompt=11 completion=3", resp.Usage)
	}
	if gotRequest.Auth != "Bearer test-key" {
		t.Fatalf("Authorization header = %q, want %q", gotRequest.Auth, "Bearer test-key")
	}
}

func TestOpenAIStreamAccumulatesToolCalls(t *testing.T) {
	sse := "data: {\"id\":\"c2\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"read\",\"arguments\":\"{\\\"pa\"}}]}}]}\n\n" +
		"data: {\"id\":\"c2\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"th\\\":\\\"a.go\\\"}\"}}]}}]}\n\n" +
		"data: {\"id\":\"c2\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n" +
		"data: [DONE]\n\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sse)
	}))
	defer srv.Close()

	p := &OpenAI{BaseURL: srv.URL, APIKey: "k", HTTPClient: srv.Client()}

	var deltas []Delta
	resp, err := p.Stream(context.Background(), Request{
		Model:    "m",
		Messages: []Message{{Role: "user", Content: "read a.go"}},
	}, func(d Delta) error {
		deltas = append(deltas, d)
		return nil
	})
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("got %d tool calls, want 1", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.ID != "call_1" || tc.Type != "function" || tc.Function.Name != "read" {
		t.Fatalf("tool call = %+v, want id=call_1 type=function name=read", tc)
	}
	wantArgs := `{"path":"a.go"}`
	if tc.Function.Arguments != wantArgs {
		t.Fatalf("arguments = %q, want %q", tc.Function.Arguments, wantArgs)
	}
}

func TestOpenAIStreamNon200ReturnsTypedAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":{"message":"boom"}}`)
	}))
	defer srv.Close()

	p := &OpenAI{BaseURL: srv.URL, APIKey: "k", HTTPClient: srv.Client()}

	_, err := p.Stream(context.Background(), Request{Model: "m"}, func(Delta) error { return nil })
	if err == nil {
		t.Fatal("Stream() expected error on 500, got nil")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("StatusCode = %d, want 500", apiErr.StatusCode)
	}
	wantBody := `{"error":{"message":"boom"}}`
	if apiErr.Body != wantBody {
		t.Fatalf("Body = %q, want %q", apiErr.Body, wantBody)
	}
}

func TestOpenAIStreamOnDeltaErrorAborts(t *testing.T) {
	sse := "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"x\"}}]}\n\n" +
		"data: [DONE]\n\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sse)
	}))
	defer srv.Close()

	p := &OpenAI{BaseURL: srv.URL, HTTPClient: srv.Client()}
	sentinel := errors.New("stop streaming")

	_, err := p.Stream(context.Background(), Request{Model: "m"}, func(Delta) error {
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("Stream() error = %v, want onDelta error %v", err, sentinel)
	}
}

func TestOpenAIStreamContextCanceled(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"x\"}}]}\n\n")
		w.(http.Flusher).Flush()
		<-ctx.Done() // long stream; the client must abort via ctx
	}))
	srv.Start()
	defer srv.Close()

	p := &OpenAI{BaseURL: srv.URL, HTTPClient: srv.Client()}
	_, err := p.Stream(ctx, Request{Model: "m"}, func(Delta) error { return nil })
	if err == nil {
		t.Fatal("Stream() expected error after context deadline, got nil")
	}
}
