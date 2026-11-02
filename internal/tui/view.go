package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	userStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("4")) // blue
	assistantStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // green
	toolCardStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("6")) // cyan
	toolResultStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8")) // gray
	errorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("1")) // red
)

// ViewLines renders each chat line with its role styling.
func (m Model) ViewLines() []string {
	out := make([]string, 0, len(m.chatLines))
	for _, l := range m.chatLines {
		switch l.role {
		case roleUser:
			out = append(out, userStyle.Render("you › "+l.text))
		case roleAssistant:
			out = append(out, assistantStyle.Render("bot › "+l.text))
		case roleToolCall:
			out = append(out, toolCardStyle.Render(l.text))
		case roleToolResult:
			out = append(out, toolResultStyle.Render(l.text))
		case roleError:
			out = append(out, errorStyle.Render(l.text))
		default:
			out = append(out, l.text)
		}
	}
	return out
}

// View renders the full chat screen: history viewport, spinner, input.
func (m Model) View() string {
	var b strings.Builder
	b.WriteString("tabdl — coding agent (ctrl+c to quit)\n")

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
