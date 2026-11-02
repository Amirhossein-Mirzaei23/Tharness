package tui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/amir/tabdl/internal/agent"
)

// Init starts the textarea blink loop.
func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles key input, resize, spinner ticks, and agent events.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.setWindowSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		case tea.KeyEnter:
			if m.state == stateRunning {
				return m, nil // ignore submissions while the agent runs
			}
			return m.submitPrompt()
		}

	case spinner.TickMsg:
		if m.state != stateRunning {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case agentEventMsg:
		return m.handleAgentEvent(msg.ev)

	case agentDoneMsg:
		if m.state == stateRunning {
			m.finalize(nil)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// spinnerTickMsgAlias removed; Update matches spinner.TickMsg directly.

// submitPrompt records the user line, flips to running, and starts the agent.
func (m Model) submitPrompt() (tea.Model, tea.Cmd) {
	prompt := strings.TrimSpace(m.input.Value())
	if prompt == "" {
		return m, nil
	}
	m.chatLines = append(m.chatLines, chatLine{role: roleUser, text: prompt})
	m.input.Reset()
	m.state = stateRunning
	m.streamIdx = -1
	m.refreshViewport()

	if m.runner == nil {
		return m, nil // no agent wired (tests); state change only
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	ch, _ := m.runner.Run(ctx, prompt)
	m.events = ch
	return m, tea.Batch(waitAgentEvent(m.events), m.spinner.Tick)
}

// agentDoneMsg signals that the active run's event channel closed.
type agentDoneMsg struct{}

// handleAgentEvent applies one agent-loop event to the transcript.
func (m Model) handleAgentEvent(ev agent.Event) (tea.Model, tea.Cmd) {
	switch {
	case ev.Err != nil:
		m.finalize(ev.Err)
		return m, m.nextWait()

	case ev.Kind == agent.EventDelta:
		if m.streamIdx < 0 {
			m.chatLines = append(m.chatLines, chatLine{role: roleAssistant})
			m.streamIdx = len(m.chatLines) - 1
		}
		m.chatLines[m.streamIdx].text += ev.Delta

	case ev.Kind == agent.EventToolCall && ev.ToolCall != nil:
		m.streamIdx = -1 // subsequent text opens a fresh assistant line
		m.chatLines = append(m.chatLines, chatLine{
			role: roleToolCall,
			text: toolCardText(ev.ToolCall.Function.Name, ev.ToolCall.Function.Arguments),
		})

	case ev.Kind == agent.EventToolResult:
		m.chatLines = append(m.chatLines, chatLine{
			role: roleToolResult,
			text: toolResultText(ev.ToolName, ev.ToolResult),
		})

	case ev.Kind == agent.EventTurnEnd:
		m.finalize(nil)
		return m, tea.Batch(m.input.Focus(), m.nextWait())
	}
	m.refreshViewport()
	return m, m.nextWait()
}

// nextWait re-arms the event wait if a run is still active.
func (m *Model) nextWait() tea.Cmd {
	if m.events == nil || m.state != stateRunning {
		return nil
	}
	return waitAgentEvent(m.events)
}

// finalize ends the current run: back to idle, stream closed, error shown.
func (m *Model) finalize(err error) {
	m.state = stateIdle
	m.streamIdx = -1
	m.cancel = nil
	m.events = nil
	if err != nil {
		m.chatLines = append(m.chatLines, chatLine{role: roleError, text: "error: " + err.Error()})
	}
	m.refreshViewport()
}

// setWindowSize records the terminal size and (re)creates the history
// viewport, reserving rows for the header and the input box.
func (m *Model) setWindowSize(w, h int) {
	m.width, m.height = w, h
	inputRows := 3 // textarea height + padding
	headerRows := 2
	vpHeight := h - inputRows - headerRows
	if vpHeight < 1 {
		vpHeight = 1
	}
	if m.history == nil {
		vp := viewport.New(w, vpHeight)
		m.history = &vp
	} else {
		m.history.Width = w
		m.history.Height = vpHeight
	}
	m.input.SetWidth(w)
	m.refreshViewport()
}

// waitAgentEvent polls the agent event channel; a closed channel yields
// agentDoneMsg so update() can finalize the run.
func waitAgentEvent(ch <-chan agent.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return agentDoneMsg{}
		}
		return agentEventMsg{ev: ev}
	}
}
