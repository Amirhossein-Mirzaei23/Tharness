package tui

import (
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/amir/tabdl/internal/agent"
)

// fakeRunner records prompts and replays scripted events on submit.
type fakeRunner struct {
	prompts []string
	events  chan agent.Event
	done    chan struct{}
	script  []agent.Event
}

func newFakeRunner(script ...agent.Event) *fakeRunner {
	return &fakeRunner{
		events: make(chan agent.Event, len(script)+1),
		done:   make(chan struct{}),
		script: script,
	}
}

func (r *fakeRunner) Run(ctx context.Context, prompt string) (<-chan agent.Event, context.CancelFunc) {
	r.prompts = append(r.prompts, prompt)
	go func() {
		defer close(r.events)
		defer close(r.done)
		for _, ev := range r.script {
			select {
			case r.events <- ev:
			case <-ctx.Done():
				return
			}
		}
	}()
	cancel := func() {} // events close on their own; ctx aborts the goroutine
	return r.events, cancel
}

// drain pumps all scripted events through Update until the run finalizes.
func drain(t *testing.T, m Model, r *fakeRunner) Model {
	t.Helper()
	for {
		select {
		case ev, ok := <-r.events:
			if !ok {
				updated, _ := m.Update(agentDoneMsg{})
				return updated.(Model)
			}
			updated, _ := m.Update(agentEventMsg{ev: ev})
			m = updated.(Model)
		case <-time.After(2 * time.Second):
			t.Fatal("timeout draining agent events")
		}
	}
}

func submit(t *testing.T, m Model, prompt string) Model {
	t.Helper()
	m.input.SetValue(prompt)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return updated.(Model)
}

// RED: submitting with a wired runner calls Run with the prompt, streams the
// scripted events into the transcript, and returns to idle on channel close.
func TestSubmitWiresRunnerAndStreamsScript(t *testing.T) {
	runner := newFakeRunner(
		agent.Event{Kind: agent.EventDelta, Delta: "ans"},
		agent.Event{Kind: agent.EventTurnEnd},
	)
	m := submit(t, New(textarea.New(), runner), "do it")

	if len(runner.prompts) != 1 || runner.prompts[0] != "do it" {
		t.Fatalf("runner prompts = %v, want [do it]", runner.prompts)
	}

	m = drain(t, m, runner)

	if m.state != stateIdle {
		t.Errorf("state = %v, want idle after stream ends", m.state)
	}
	last := m.chatLines[len(m.chatLines)-1]
	if last.role != roleAssistant || last.text != "ans" {
		t.Errorf("last line = {%q %q}, want {assistant ans}", last.role, last.text)
	}
}

// RED: the runner goroutine lifecycle is sound — done is closed after the run.
func TestRunnerDoneSignalledAfterScript(t *testing.T) {
	runner := newFakeRunner(agent.Event{Kind: agent.EventTurnEnd})
	m := submit(t, New(textarea.New(), runner), "go")
	m = drain(t, m, runner)

	select {
	case <-runner.done:
	default:
		t.Error("runner done channel not closed after script finished")
	}
	if m.state != stateIdle {
		t.Errorf("state = %v, want idle", m.state)
	}
}
