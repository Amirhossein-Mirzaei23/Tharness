package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Amirhossein-Mirzaei23/Tharness/internal/agent"
	"github.com/Amirhossein-Mirzaei23/Tharness/internal/ai"
)

func newTestModel() Model {
	ta := textarea.New()
	ta.Placeholder = "Type a message…"
	ta.Focus()
	return New(ta, nil)
}

// RED: a fresh model shows the chat placeholder and the input prompt.
func TestInitialViewShowsPlaceholderAndInput(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "tharness") {
		t.Errorf("View() = %q, want it to contain %q", view, "tharness")
	}
	if !strings.Contains(view, "Type a message") {
		t.Errorf("View() = %q, want it to contain the textarea placeholder", view)
	}
	if !strings.Contains(view, textareaPrompt) {
		t.Errorf("View() = %q, want it to contain prompt %q", view, textareaPrompt)
	}
}

// RED: submitting (enter) adds a user line and flips the model to running.
func TestEnterSubmitsUserLineAndEntersRunning(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("hello agent")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if got := m.input.Value(); got != "" {
		t.Errorf("input.Value() = %q after submit, want empty", got)
	}
	lines := nonEmptyLines(m)
	if len(lines) == 0 || !strings.Contains(lines[len(lines)-1], "hello agent") {
		t.Errorf("last line = %v, want a user line containing %q", lines, "hello agent")
	}
	if m.state != stateRunning {
		t.Errorf("state = %v, want %v", m.state, stateRunning)
	}
	if len(m.chatLines) == 0 || m.chatLines[len(m.chatLines)-1].role != "user" {
		t.Errorf("last chatLine role = %q, want %q", lastRole(m), "user")
	}
}

// RED: a Delta event appends text to the current assistant line.
func TestDeltaAppendsToAssistantLine(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("hi")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	updated, _ = m.Update(agentEventMsg{ev: agent.Event{Kind: agent.EventDelta, Delta: "Hel"}})
	m = updated.(Model)
	updated, _ = m.Update(agentEventMsg{ev: agent.Event{Kind: agent.EventDelta, Delta: "lo"}})
	m = updated.(Model)

	last := m.chatLines[len(m.chatLines)-1]
	if last.role != "assistant" || last.text != "Hello" {
		t.Errorf("last line = {%q %q}, want {assistant Hello}", last.role, last.text)
	}
	if m.state != stateRunning {
		t.Errorf("state = %v, want %v (delta must not change state)", m.state, stateRunning)
	}
}

// RED: a ToolCall event adds a "⚙ name(args)" card line; ToolResult adds a
// result preview line.
func TestToolCallAndResultAddCardLines(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("run something")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	call := &ai.ToolCall{ID: "c1", Type: "function"}
	call.Function.Name = "bash"
	call.Function.Arguments = `{"command":"ls -la"}`
	updated, _ = m.Update(agentEventMsg{ev: agent.Event{Kind: agent.EventToolCall, ToolCall: call}})
	m = updated.(Model)

	n := len(m.chatLines)
	last := m.chatLines[n-1]
	if last.role != roleToolCall {
		t.Errorf("last role = %q, want %q", last.role, roleToolCall)
	}
	if !strings.Contains(last.text, "⚙") || !strings.Contains(last.text, "bash") || !strings.Contains(last.text, "ls -la") {
		t.Errorf("tool-call card text = %q, want ⚙ + name + args", last.text)
	}

	updated, _ = m.Update(agentEventMsg{ev: agent.Event{Kind: agent.EventToolResult, ToolName: "bash", ToolResult: "file1.go\nfile2.go"}})
	m = updated.(Model)

	last = m.chatLines[len(m.chatLines)-1]
	if last.role != roleToolResult {
		t.Errorf("last role = %q, want %q", last.role, roleToolResult)
	}
	if !strings.Contains(last.text, "file1.go") {
		t.Errorf("tool-result card text = %q, want it to contain the result preview", last.text)
	}
	if strings.Contains(last.text, "file2.go") {
		t.Errorf("tool-result card text = %q, want the preview truncated to one line", last.text)
	}
}

// RED: TurnEnd finalizes the turn — model returns to idle and input re-focuses.
func TestTurnEndReturnsToIdle(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("hi")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	updated, _ = m.Update(agentEventMsg{ev: agent.Event{Kind: agent.EventDelta, Delta: "done"}})
	m = updated.(Model)

	updated, cmd := m.Update(agentEventMsg{ev: agent.Event{Kind: agent.EventTurnEnd}})
	m = updated.(Model)

	if m.state != stateIdle {
		t.Errorf("state = %v, want %v", m.state, stateIdle)
	}
	if !m.input.Focused() {
		t.Error("input not focused after TurnEnd, want it re-focused for the next prompt")
	}
	if cmd == nil {
		t.Error("TurnEnd returned nil cmd, want waitForAgentEvent (agent may run again)")
	}
}

// RED: ctrl+c aborts — quit command returned, agent context cancelled.
func TestCtrlCQuitsAndCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := newTestModel()
	m.cancel = cancel

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(Model)

	if cmd == nil {
		t.Fatal("ctrl+c returned nil cmd, want tea.Quit")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("ctrl+c cmd produced %T, want tea.QuitMsg", msg)
	}
	select {
	case <-ctx.Done():
	default:
		t.Error("ctrl+c did not cancel the agent context")
	}
}

// RED: an agent error event finalizes the run, shows the error, returns to idle.
func TestAgentErrorFinalizesRun(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("hi")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	updated, _ = m.Update(agentEventMsg{ev: agent.Event{Kind: agent.EventTurnEnd, Err: errors.New("boom")}})
	m = updated.(Model)

	if m.state != stateIdle {
		t.Errorf("state = %v, want %v", m.state, stateIdle)
	}
	found := false
	for _, l := range m.chatLines {
		if strings.Contains(l.text, "boom") {
			found = true
		}
	}
	if !found {
		t.Errorf("chat lines %v do not mention the error", m.chatLines)
	}
}

// RED: enter while the agent is running is ignored (no duplicate submissions).
func TestEnterIgnoredWhileRunning(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("first")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	m.input.SetValue("second")
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.input.Value() != "second" {
		t.Errorf("input = %q, want %q (submit must be ignored while running)", m.input.Value(), "second")
	}
	userLines := 0
	for _, l := range m.chatLines {
		if l.role == "user" {
			userLines++
		}
	}
	if userLines != 1 {
		t.Errorf("user lines = %d, want 1", userLines)
	}
}

// RED: spinner renders while running.
func TestSpinnerWhileRunning(t *testing.T) {
	m := newTestModel()
	if strings.Contains(m.View(), m.spinner.Spinner.Frames[0]) {
		t.Error("spinner visible while idle, want it only while running")
	}
	m.state = stateRunning
	if !strings.Contains(m.View(), m.spinner.Spinner.Frames[0]) {
		t.Error("spinner frame missing from view while running")
	}
}

// RED: viewport renders chat history and auto-scrolls to the bottom.
func TestViewportRendersHistoryAndScrolls(t *testing.T) {
	m := newTestModel()
	m.chatLines = []chatLine{
		{role: "user", text: "first question"},
		{role: "assistant", text: "first answer"},
	}
	m.setWindowSize(80, 24)
	m.refreshViewport()

	view := m.View()
	if !strings.Contains(view, "first question") {
		t.Errorf("View() = %q, want chat history rendered", view)
	}
	if vp := m.history; vp != nil {
		if !vp.AtBottom() {
			t.Error("viewport not at bottom after refresh, want auto-scroll")
		}
	} else {
		t.Error("history viewport is nil")
	}
}

func nonEmptyLines(m Model) []string {
	var out []string
	for _, l := range m.ViewLines() {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

func lastRole(m Model) string {
	if len(m.chatLines) == 0 {
		return ""
	}
	return m.chatLines[len(m.chatLines)-1].role
}
