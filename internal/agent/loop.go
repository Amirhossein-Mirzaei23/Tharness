package agent

import (
	"context"
	"fmt"

	"github.com/amir/tabdl/internal/ai"
	"github.com/amir/tabdl/internal/tools"
)

const defaultMaxTurns = 25

// Loop drives turn-taking between the model and tool execution.
type Loop struct {
	Provider ai.Provider
	Model    string
	Tools    *tools.Registry
	MaxTurns int // defaults to 25 when zero

	// Compactor, when non-nil, runs against the history before each provider
	// call (in-loop compaction). When the estimated token count of the
	// history exceeds the compactor's threshold, older messages are replaced
	// by a summary before the request is sent. A nil Compactor disables
	// compaction; a Compactor with ThresholdTokens <= 0 never compacts.
	Compactor *Compactor
}

// Run executes the agent loop starting from msgs and returns the full
// resulting message history. emit, when non-nil, receives progress events.
// MaxTurns limits the number of provider calls; exceeding it is an error.
// Context cancellation aborts the loop with the context's error.
func (l *Loop) Run(ctx context.Context, msgs []ai.Message, emit func(Event)) ([]ai.Message, error) {
	maxTurns := l.MaxTurns
	if maxTurns <= 0 {
		maxTurns = defaultMaxTurns
	}

	history := make([]ai.Message, len(msgs))
	copy(history, msgs)

	emitEvent := func(e Event) {
		if emit != nil {
			emit(e)
		}
	}

	for turn := 0; turn < maxTurns; turn++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		emitEvent(Event{Kind: EventTurnStart})

		if l.Compactor != nil {
			compacted, didCompact, err := l.Compactor.MaybeCompact(ctx, history)
			if err != nil {
				return nil, err
			}
			if didCompact {
				history = compacted
			}
		}

		req := ai.Request{
			Model:    l.Model,
			Messages: history,
			Tools:    registryTools(l.Tools),
		}
		resp, err := l.Provider.Stream(ctx, req, func(d ai.Delta) error {
			emitEvent(Event{Kind: EventDelta, Delta: d.Content})
			return nil
		})
		if err != nil {
			return nil, err
		}

		history = append(history, ai.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		if len(resp.ToolCalls) == 0 {
			emitEvent(Event{Kind: EventTurnEnd})
			return history, nil
		}

		for _, tc := range resp.ToolCalls {
			emitEvent(Event{Kind: EventToolCall, ToolCall: &tc})

			result := l.execTool(ctx, tc)
			emitEvent(Event{Kind: EventToolResult, ToolName: tc.Function.Name, ToolResult: result})

			history = append(history, ai.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}
	return nil, fmt.Errorf("agent: max turns (%d) exceeded", maxTurns)
}

func (l *Loop) execTool(ctx context.Context, tc ai.ToolCall) string {
	if l.Tools == nil {
		return fmt.Sprintf("error: unknown tool %s", tc.Function.Name)
	}
	res, err := l.Tools.Exec(ctx, tc.Function.Name, tc.Function.Arguments)
	if err != nil {
		return fmt.Sprintf("error: %s", err)
	}
	return res
}

// registryTools converts registry tools to provider request form. A nil
// registry yields no tools.
func registryTools(reg *tools.Registry) []ai.Tool {
	if reg == nil {
		return nil
	}
	var out []ai.Tool
	for _, name := range reg.Names() {
		t, ok := reg.Get(name)
		if !ok {
			continue
		}
		out = append(out, ai.Tool{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
	}
	return out
}
