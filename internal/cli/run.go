// Package cli implements the tabdl command-line interface.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/amir/tabdl/internal/agent"
	"github.com/amir/tabdl/internal/ai"
	"github.com/amir/tabdl/internal/config"
	"github.com/amir/tabdl/internal/session"
	"github.com/amir/tabdl/internal/tools"
)

// RunOptions configures a one-shot (`-p`) invocation.
type RunOptions struct {
	// Prompt is the user's request; required.
	Prompt string
	// ConfigPath overrides the config file location (empty = default).
	ConfigPath string
	// DataDir overrides the config data dir (used by tests).
	DataDir string
	// BaseURL overrides the provider endpoint (used by tests).
	BaseURL string
	// APIKey overrides the resolved API key (used by tests).
	APIKey string
	// Tools, when non-nil, replaces the default core tool registry.
	Tools *tools.Registry
	// Resume, when non-empty, is the id of a stored session whose messages
	// are loaded before Prompt is appended.
	Resume string
}

// Run executes the one-shot pipeline: config → provider → tools → loop →
// stdout, appending the session transcript to <DataDir>/sessions/<id>.jsonl.
func (o RunOptions) Run(ctx context.Context, stdout io.Writer) error {
	cfg, err := config.Load(o.ConfigPath)
	if err != nil {
		return fmt.Errorf("cli: load config: %w", err)
	}
	if o.DataDir != "" {
		cfg.DataDir = o.DataDir
	}
	dataDir := firstNonEmpty(os.Getenv("TABDL_DATA_DIR"), o.DataDir, cfg.DataDir)
	baseURL := firstNonEmpty(os.Getenv("TABDL_BASE_URL"), cfg.BaseURL, o.BaseURL, "https://api.z.ai/api/paas/v4")
	apiKey := firstNonEmpty(os.Getenv("TABDL_API_KEY"), o.APIKey, envNamed(cfg.APIKeyEnv))

	store, err := session.Open(dataDir)
	if err != nil {
		return err
	}

	// Seed messages: prior session (resume) + fresh user prompt.
	var msgs []ai.Message
	if o.Resume != "" {
		events, _, err := store.Load(o.Resume)
		if err != nil {
			return fmt.Errorf("cli: resume session %q: %w", o.Resume, err)
		}
		msgs = session.ResumeToMessages(events)
	}
	msgs = append(msgs, ai.Message{Role: "user", Content: o.Prompt})

	provider := &ai.OpenAI{BaseURL: baseURL, APIKey: apiKey}
	registry := o.Tools
	if registry == nil {
		registry = tools.NewCoreRegistry()
	}

	loop := &agent.Loop{Provider: provider, Model: cfg.Model, Tools: registry}

	_, path, err := store.New()
	if err != nil {
		return err
	}
	appendEvent := func(ev session.Event) {
		if ev.TS == "" {
			ev.TS = time.Now().UTC().Format(time.RFC3339)
		}
		ev.Type = "message"
		_ = store.Append(path, ev) //nolint:errcheck // transcript best-effort
	}

	// Record the seeding messages (resume history + prompt).
	for _, m := range msgs {
		appendEvent(session.Event{Role: m.Role, Content: m.Content})
	}

	final, err := loop.Run(ctx, msgs, nil)
	if err != nil {
		return fmt.Errorf("cli: run: %w", err)
	}

	// Record the resulting history (assistant/tool messages beyond seeding).
	for _, m := range final[len(msgs):] {
		ev := session.Event{Role: m.Role, Content: m.Content, ToolCalls: toSessionToolCalls(m.ToolCalls), ToolCallID: m.ToolCallID}
		if m.Role == "tool" {
			ev.ToolResult = m.Content
			if tc := toolCallFor(m.ToolCallID, final); tc != nil {
				ev.ToolName = tc.Function.Name
				ev.Content = tc.Function.Arguments
			}
		}
		appendEvent(ev)
	}

	// The last message is the final assistant answer.
	if n := len(final); n > 0 {
		fmt.Fprintln(stdout, final[n-1].Content)
	}
	return nil
}

// toSessionToolCalls converts ai tool calls to transcript form.
func toSessionToolCalls(tcs []ai.ToolCall) []session.ToolCall {
	if len(tcs) == 0 {
		return nil
	}
	out := make([]session.ToolCall, 0, len(tcs))
	for _, tc := range tcs {
		var sc session.ToolCall
		sc.ID = tc.ID
		sc.Type = tc.Type
		sc.Function.Name = tc.Function.Name
		sc.Function.Arguments = tc.Function.Arguments
		out = append(out, sc)
	}
	return out
}

// toolCallFor finds the assistant tool call with the given call ID.
func toolCallFor(callID string, history []ai.Message) *ai.ToolCall {
	for _, m := range history {
		for i, tc := range m.ToolCalls {
			if tc.ID == callID {
				return &m.ToolCalls[i]
			}
		}
	}
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func envNamed(name string) string {
	if name == "" {
		return ""
	}
	return os.Getenv(name)
}
