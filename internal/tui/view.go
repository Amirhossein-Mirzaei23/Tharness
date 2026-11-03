package tui

import (
	"strings"
)

// ViewLines renders each chat line with its role styling.
func (m Model) ViewLines() []string {
	out := make([]string, 0, len(m.chatLines))
	for _, l := range m.chatLines {
		switch l.role {
		case roleUser:
			out = append(out, m.styles.User.Render("you › "+l.text))
		case roleAssistant:
			out = append(out, m.styles.Assistant.Render("bot › "+l.text))
		case roleToolCall:
			out = append(out, m.styles.Tool.Render(l.text))
		case roleToolResult:
			out = append(out, m.styles.ToolResult.Render(l.text))
		case roleError:
			out = append(out, m.styles.Error.Render(l.text))
		default:
			out = append(out, l.text)
		}
	}
	return out
}

// View renders the full chat screen: history viewport, spinner, input.
func (m Model) View() string {
	var b strings.Builder
	b.WriteString(m.styles.Accent.Render("tharness — coding agent (ctrl+c to quit)") + "\n")

	if m.history != nil {
		b.WriteString(m.history.View())
		b.WriteString("\n")
	} else {
		for _, l := range m.ViewLines() {
			b.WriteString(l)
			b.WriteString("\n")
		}
	}

	if m.state == stateRunning {
		b.WriteString(m.spinner.View())
		b.WriteString("\n")
	}
	b.WriteString(m.input.View())
	return b.String()
}

// refreshViewport pushes the current transcript into the history viewport
// and scrolls to the bottom.
func (m *Model) refreshViewport() {
	if m.history == nil {
		return
	}
	m.history.SetContent(strings.Join(m.ViewLines(), "\n"))
	m.history.GotoBottom()
}
