package tui

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Amirhossein-Mirzaei23/Tharness/internal/agent"
	"github.com/Amirhossein-Mirzaei23/Tharness/internal/ai"
	"github.com/Amirhossein-Mirzaei23/Tharness/internal/config"
	"github.com/Amirhossein-Mirzaei23/Tharness/internal/session"
	"github.com/Amirhossein-Mirzaei23/Tharness/internal/tools"
)

// ChatRunner is the production AgentRunner: it resolves configuration the
// same way the one-shot CLI does and drives one agent.Loop per prompt,
// streaming Events over a channel and appending the transcript to
// <DataDir>/sessions/.
type ChatRunner struct {
	provider ai.Provider
	model    string
	theme    string
	tools    *tools.Registry
	store    *session.Store
	path     string
}

// NewChatRunner resolves config → provider → tools → store. configPath
// overrides the config file location (empty = default). Provider settings
// follow ai.ResolveProvider precedence: THARNESS_BASE_URL / THARNESS_API_KEY
// > per-provider config entry > provider env var > legacy flat config.
func NewChatRunner(configPath string) (*ChatRunner, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("tui: load config: %w", err)
	}
	dataDir := cfg.DataDir
	if v := os.Getenv("THARNESS_DATA_DIR"); v != "" {
		dataDir = v
	}
	resolved, err := ai.ResolveProvider(cfg, os.Getenv)
	if err != nil {
		return nil, fmt.Errorf("tui: %w", err)
	}

	store, err := session.Open(dataDir)
	if err != nil {
		return nil, err
	}
	_, path, err := store.New()
	if err != nil {
		return nil, err
	}

	registry := tools.NewCoreRegistry()
	registry.Register(tools.NewBrowserTool(cfg.Browser.Command))

	return &ChatRunner{
		provider: &ai.OpenAI{BaseURL: resolved.BaseURL, APIKey: resolved.APIKey},
		model:    resolved.Model,
		theme:    cfg.Theme,
		tools:    registry,
		store:    store,
		path:     path,
	}, nil
}

// Theme returns the configured TUI theme name.
func (r *ChatRunner) Theme() string { return r.theme }

// Run starts one agent run; implements tui.AgentRunner. Events are fed to the
// returned channel; the channel is closed when the run finishes. The returned
// cancel func aborts the run (ctrl+c).
func (r *ChatRunner) Run(ctx context.Context, prompt string) (<-chan agent.Event, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)
	events := make(chan agent.Event)

	msgs := []ai.Message{{Role: "user", Content: prompt}}
	r.append(session.Event{Role: "user", Content: prompt})

	go func() {
		defer close(events)
		defer cancel()

		final, err := r.loop().Run(ctx, msgs, func(ev agent.Event) {
			events <- ev
		})
		if err != nil {
			events <- agent.Event{Kind: agent.EventTurnEnd, Err: err}
			return
		}
		for _, m := range final[len(msgs):] {
			r.append(session.Event{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID})
		}
	}()

	return events, cancel
}

// loop builds a fresh agent Loop per run (stateless between prompts).
func (r *ChatRunner) loop() *agent.Loop {
	return &agent.Loop{Provider: r.provider, Model: r.model, Tools: r.tools}
}

// append best-effort records one transcript event.
func (r *ChatRunner) append(ev session.Event) {
	if ev.TS == "" {
		ev.TS = time.Now().UTC().Format(time.RFC3339)
	}
	ev.Type = "message"
	_ = r.store.Append(r.path, ev) //nolint:errcheck // transcript best-effort
}
