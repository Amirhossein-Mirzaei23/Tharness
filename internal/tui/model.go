// Package tui implements the tharness bubbletea chat interface.
package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"

	"github.com/Amirhossein-Mirzaei23/Tharness/internal/agent"
	"github.com/Amirhossein-Mirzaei23/Tharness/internal/config"
	"github.com/Amirhossein-Mirzaei23/Tharness/internal/theme"
)

// textareaPrompt prefixes the input box.
const textareaPrompt = "> "

// chatLine is one rendered line of the conversation.
type chatLine struct {
	role string // "user", "assistant", roleToolCall, roleToolResult, roleError
	text string
}

const (
	roleUser       = "user"
	roleAssistant  = "assistant"
	roleToolCall   = "tool_call"
	roleToolResult = "tool_result"
	roleError      = "error"
)

type uiState int

const (
	stateIdle uiState = iota
	stateRunning
)

// agentEventMsg wraps one agent-loop Event for the bubbletea update loop.
type agentEventMsg struct {
	ev agent.Event
}

// AgentRunner starts one agent run in the background. Run must feed agent
// Events to the returned channel and close it when the run finishes
// (success or failure). The returned cancel func aborts the run.
type AgentRunner interface {
	Run(ctx context.Context, prompt string) (<-chan agent.Event, context.CancelFunc)
}

// Model is the bubbletea chat model.
type Model struct {
	// runner starts agent runs; nil disables submission (tests).
	runner AgentRunner
	// chatLines is the full conversation transcript as display lines.
	chatLines []chatLine
	// streamIdx is the index of the assistant line currently receiving
	// deltas, or -1 when no stream is open.
	streamIdx int
	// state tracks whether the agent is idle or running.
	state uiState
	// events is the active run's event channel (nil when idle).
	events <-chan agent.Event
	// cancel aborts the running agent.
	cancel context.CancelFunc

	input   textarea.Model
	spinner spinner.Model
	history *viewport.Model
	styles  theme.Styles
	width   int
	height  int
}

// New returns a chat Model over the given input component. runner may be nil
// in tests; a nil runner still records submissions but starts no agent. The
// model starts with the default (dark) theme; call WithTheme to switch.
func New(input textarea.Model, runner AgentRunner) Model {
	input.Prompt = textareaPrompt
	input.Focus()
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return Model{
		input:     input,
		spinner:   sp,
		streamIdx: -1,
		runner:    runner,
		styles:    theme.StylesFor(config.DefaultTheme),
	}
}

// WithTheme returns the model restyled for the named theme; unknown theme
// names keep the current styling.
func (m Model) WithTheme(name string) Model {
	if _, ok := theme.Named(name); ok {
		m.styles = theme.StylesFor(name)
	}
	return m
}
