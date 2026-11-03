package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/Amirhossein-Mirzaei23/Tharness/internal/ai"
)

func TestEstimateTokensSumsContentCharsDividedByFour(t *testing.T) {
	msgs := []ai.Message{
		{Role: "user", Content: "12345678"},          // 8 chars → 2
		{Role: "assistant", Content: "abcdefghijkl"}, // 12 chars → 3
		{Role: "tool", Content: "xyz"},               // 3 chars → 0 (integer div)
	}
	// (8+12+3)/4 = 23/4 = 5
	if got := EstimateTokens(msgs); got != 5 {
		t.Errorf("EstimateTokens = %d, want 5", got)
	}
	if got := EstimateTokens(nil); got != 0 {
		t.Errorf("EstimateTokens(nil) = %d, want 0", got)
	}
}

func TestMaybeCompactBelowThresholdReturnsUnchanged(t *testing.T) {
	p := newFakeProvider(ai.Response{Content: "should not be called"})
	c := &Compactor{Provider: p, Model: "test-model", ThresholdTokens: 1000, KeepRecent: 2}
	msgs := []ai.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}

	got, compacted, err := c.MaybeCompact(context.Background(), msgs)
	if err != nil {
		t.Fatalf("MaybeCompact: unexpected error: %v", err)
	}
	if compacted {
		t.Error("compacted = true, want false below threshold")
	}
	if len(got) != len(msgs) {
		t.Fatalf("returned %d messages, want %d (unchanged)", len(got), len(msgs))
	}
	for i := range msgs {
		if got[i].Role != msgs[i].Role || got[i].Content != msgs[i].Content {
			t.Errorf("got[%d] = %+v, want %+v", i, got[i], msgs[i])
		}
	}
	if p.callCount() != 0 {
		t.Errorf("provider called %d times, want 0 below threshold", p.callCount())
	}
}

func TestMaybeCompactZeroThresholdNeverCompacts(t *testing.T) {
	p := newFakeProvider(ai.Response{Content: "should not be called"})
	c := &Compactor{Provider: p, Model: "test-model", ThresholdTokens: 0, KeepRecent: 2}
	msgs := []ai.Message{{Role: "user", Content: strings.Repeat("a", 100000)}}

	got, compacted, err := c.MaybeCompact(context.Background(), msgs)
	if err != nil {
		t.Fatalf("MaybeCompact: unexpected error: %v", err)
	}
	if compacted {
		t.Error("compacted = true, want false when ThresholdTokens <= 0")
	}
	if len(got) != len(msgs) {
		t.Errorf("returned %d messages, want %d", len(got), len(msgs))
	}
	if p.callCount() != 0 {
		t.Errorf("provider called %d times, want 0", p.callCount())
	}
}

func TestMaybeCompactAboveThresholdCallsProviderWithSummarizePrompt(t *testing.T) {
	p := newFakeProvider(ai.Response{Content: "summary text"})
	c := &Compactor{Provider: p, Model: "test-model", ThresholdTokens: 4, KeepRecent: 1}
	msgs := []ai.Message{
		{Role: "user", Content: "aaaa"},      // 4 chars
		{Role: "assistant", Content: "bbbb"}, // 4 chars → total 2 tokens ≥ 4? 8/4=2 < 4...
	}

	// Make it clearly above threshold: 3 messages of 8 chars = 24/4 = 6 tokens.
	msgs = append(msgs, ai.Message{Role: "user", Content: "cccccccc"}) // 8 chars → 32/4 = 8
	_, compacted, err := c.MaybeCompact(context.Background(), msgs)
	if err != nil {
		t.Fatalf("MaybeCompact: unexpected error: %v", err)
	}
	if !compacted {
		t.Fatal("compacted = false, want true above threshold")
	}
	if p.callCount() != 1 {
		t.Fatalf("provider called %d times, want 1", p.callCount())
	}
	req := p.requests[0]
	if req.Model != "test-model" {
		t.Errorf("request model = %q, want test-model", req.Model)
	}
	if len(req.Messages) != 1 {
		t.Fatalf("summarize request has %d messages, want 1", len(req.Messages))
	}
	if req.Messages[0].Role != "user" {
		t.Errorf("summarize prompt role = %q, want user", req.Messages[0].Role)
	}
	// The prompt must contain the older portion rendered as transcript text.
	if !strings.Contains(req.Messages[0].Content, "summar") {
		t.Errorf("summarize prompt %q does not mention summarizing", req.Messages[0].Content)
	}
	// The prompt must contain the older portion rendered as transcript text,
	// but not the most recent message (protected by KeepRecent=1).
	if !strings.Contains(req.Messages[0].Content, "aaaa") || !strings.Contains(req.Messages[0].Content, "bbbb") {
		t.Errorf("summarize prompt does not include older messages: %q", req.Messages[0].Content)
	}
	if strings.Contains(req.Messages[0].Content, "cccccccc") {
		t.Errorf("summarize prompt includes recent message despite KeepRecent=1: %q", req.Messages[0].Content)
	}
}

func TestMaybeCompactReplacesOlderWithSummaryKeepsRecentVerbatim(t *testing.T) {
	p := newFakeProvider(ai.Response{Content: "the key facts"})
	c := &Compactor{Provider: p, Model: "test-model", ThresholdTokens: 4, KeepRecent: 2}
	msgs := []ai.Message{
		{Role: "user", Content: "aaaaaaaa"},
		{Role: "assistant", Content: "aaaaaaaa"},
		{Role: "user", Content: "aaaaaaaa"},
		{Role: "assistant", Content: "recent-1"},
		{Role: "user", Content: "recent-2"},
	}

	got, compacted, err := c.MaybeCompact(context.Background(), msgs)
	if err != nil {
		t.Fatalf("MaybeCompact: unexpected error: %v", err)
	}
	if !compacted {
		t.Fatal("compacted = false, want true")
	}
	if len(got) != 3 {
		t.Fatalf("result length = %d, want 3 (summary + 2 recent): %+v", len(got), got)
	}
	if got[0].Role != "user" {
		t.Errorf("summary role = %q, want user", got[0].Role)
	}
	if got[0].Content != "Conversation summary: the key facts" {
		t.Errorf("summary content = %q, want prefixed provider response", got[0].Content)
	}
	if got[1].Role != "assistant" || got[1].Content != "recent-1" {
		t.Errorf("got[1] = %+v, want recent-1 verbatim", got[1])
	}
	if got[2].Role != "user" || got[2].Content != "recent-2" {
		t.Errorf("got[2] = %+v, want recent-2 verbatim", got[2])
	}
}

func TestMaybeCompactKeepRecentZeroSummarizesEverything(t *testing.T) {
	p := newFakeProvider(ai.Response{Content: "all of it"})
	c := &Compactor{Provider: p, Model: "test-model", ThresholdTokens: 4, KeepRecent: 0}
	msgs := []ai.Message{
		{Role: "user", Content: "aaaaaaaa"},
		{Role: "assistant", Content: "aaaaaaaa"},
	}

	got, compacted, err := c.MaybeCompact(context.Background(), msgs)
	if err != nil {
		t.Fatalf("MaybeCompact: unexpected error: %v", err)
	}
	if !compacted {
		t.Fatal("compacted = false, want true")
	}
	if len(got) != 1 {
		t.Fatalf("result length = %d, want 1 (summary only): %+v", len(got), got)
	}
	if got[0].Role != "user" || got[0].Content != "Conversation summary: all of it" {
		t.Errorf("got[0] = %+v, want summary message", got[0])
	}
	// The transcript in the summarize prompt must include every message.
	if !strings.Contains(p.requests[0].Messages[0].Content, "aaaaaaaa") {
		t.Errorf("summarize prompt missing message content: %q", p.requests[0].Messages[0].Content)
	}
}

func TestMaybeCompactProviderErrorPropagates(t *testing.T) {
	p := newFakeProvider() // no scripted responses → errors
	c := &Compactor{Provider: p, Model: "test-model", ThresholdTokens: 4, KeepRecent: 1}
	msgs := []ai.Message{
		{Role: "user", Content: "aaaaaaaa"},
		{Role: "assistant", Content: "aaaaaaaa"},
	}

	got, compacted, err := c.MaybeCompact(context.Background(), msgs)
	if err == nil {
		t.Fatal("MaybeCompact: err = nil, want provider error to propagate")
	}
	if compacted {
		t.Error("compacted = true, want false on error")
	}
	if got != nil {
		t.Errorf("returned messages = %+v, want nil on error", got)
	}
}

func TestLoopRunsCompactorBeforeFirstProviderCall(t *testing.T) {
	p := newFakeProvider(ai.Response{Content: "final answer"})
	c := &Compactor{Provider: p, Model: "test-model", ThresholdTokens: 4, KeepRecent: 1}
	loop := &Loop{Provider: p, Model: "test-model", MaxTurns: 5, Compactor: c}

	history := []ai.Message{
		{Role: "user", Content: "aaaaaaaa"},
		{Role: "assistant", Content: "aaaaaaaa"},
		{Role: "user", Content: "recent"},
	}

	final, err := loop.Run(context.Background(), history, nil)
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}

	// The first provider request (the compaction call) saw the full history;
	// the second (the loop's first real call) must see the compacted history.
	if p.callCount() != 2 {
		t.Fatalf("provider called %d times, want 2 (compact + turn)", p.callCount())
	}
	first := p.requests[1].Messages
	if len(first) != 2 {
		t.Fatalf("loop request has %d messages, want 2 (summary + recent): %+v", len(first), first)
	}
	if !strings.HasPrefix(first[0].Content, "Conversation summary: ") {
		t.Errorf("loop request msg[0] = %+v, want summary message", first[0])
	}
	if first[1].Content != "recent" {
		t.Errorf("loop request msg[1] = %+v, want recent message kept verbatim", first[1])
	}
	// The loop's returned history continues from the compacted form.
	if len(final) != 3 {
		t.Fatalf("final history length = %d, want 3: %+v", len(final), final)
	}
	if final[2].Content != "final answer" {
		t.Errorf("final[2].Content = %q, want final answer", final[2].Content)
	}
}

func TestLoopWithoutCompactorSendsFullHistory(t *testing.T) {
	p := newFakeProvider(ai.Response{Content: "done"})
	loop := &Loop{Provider: p, Model: "test-model", MaxTurns: 5}

	_, err := loop.Run(context.Background(), []ai.Message{{Role: "user", Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if p.callCount() != 1 {
		t.Fatalf("provider called %d times, want 1", p.callCount())
	}
	if len(p.requests[0].Messages) != 1 || p.requests[0].Messages[0].Content != "hi" {
		t.Errorf("request messages = %+v, want original history", p.requests[0].Messages)
	}
}
