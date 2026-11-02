package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/amir/tabdl/internal/ai"
)

// Compactor summarizes older history when the estimated token count of the
// conversation exceeds ThresholdTokens.
//
// KeepRecent is the number of most-recent messages that are never
// summarized — they are always kept verbatim so the model retains immediate
// context. KeepRecent <= 0 means no messages are protected: everything may
// be summarized (the summary message itself still exists afterwards).
//
// A ThresholdTokens <= 0 disables compaction entirely: MaybeCompact never
// calls the provider and always returns the input unchanged.
type Compactor struct {
	Provider        ai.Provider
	Model           string
	ThresholdTokens int
	KeepRecent      int
}

// MaybeCompact checks the estimated token count of msgs against the
// threshold. Below the threshold (or when disabled) it returns (msgs, false,
// nil) without calling the provider. Otherwise the older portion of the
// history — everything except the last KeepRecent messages — is rendered as
// transcript text and sent to the provider with a summarize prompt; the
// provider's response replaces the older portion as a single summary
// message, and the result is returned with compacted=true.
func (c *Compactor) MaybeCompact(ctx context.Context, msgs []ai.Message) ([]ai.Message, bool, error) {
	if c.ThresholdTokens <= 0 || EstimateTokens(msgs) < c.ThresholdTokens {
		return msgs, false, nil
	}

	keep := c.KeepRecent
	if keep < 0 {
		keep = 0
	}
	if keep > len(msgs) {
		keep = len(msgs)
	}
	older := msgs[:len(msgs)-keep]
	recent := msgs[len(msgs)-keep:]

	var b strings.Builder
	b.WriteString("Summarize the following conversation transcript. ")
	b.WriteString("Produce a concise summary preserving key facts, decisions, and open tasks.\n\nTranscript:\n")
	for _, m := range older {
		fmt.Fprintf(&b, "%s: %s\n", m.Role, m.Content)
	}

	resp, err := c.Provider.Stream(ctx, ai.Request{
		Model: c.Model,
		Messages: []ai.Message{{
			Role:    "user",
			Content: b.String(),
		}},
	}, func(ai.Delta) error { return nil })
	if err != nil {
		return nil, false, err
	}

	out := make([]ai.Message, 0, 1+len(recent))
	out = append(out, ai.Message{
		Role:    "user",
		Content: "Conversation summary: " + resp.Content,
	})
	out = append(out, recent...)
	return out, true, nil
}

// EstimateTokens returns a rough token estimate for the message history:
// the total number of content characters divided by 4 (integer division).
// It is deliberately simple — a heuristic for compaction thresholds, not a
// real tokenizer.
func EstimateTokens(msgs []ai.Message) int {
	total := 0
	for _, m := range msgs {
		total += len(m.Content)
	}
	return total / 4
}
