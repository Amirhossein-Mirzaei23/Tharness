// Package theme defines the TUI color palettes selectable by
// `tharness setup`.
package theme

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Palette is one named color scheme. Colors are ANSI-16 or 256-color codes
// so every theme degrades gracefully on plain terminals.
type Palette struct {
	// Name is the config/CLI identifier.
	Name string
	// Desc is a one-line description for the setup wizard.
	Desc string
	// User, Assistant, Tool, ToolResult, Error color chat roles.
	User       string
	Assistant  string
	Tool       string
	ToolResult string
	Error      string
	// Accent colors the header and spinner-adjacent chrome.
	Accent string
	// Input colors the prompt border/input chrome.
	Input string
}

// palettes lists the built-in themes in setup order.
var palettes = []Palette{
	{
		Name: "dark", Desc: "ANSI colors tuned for dark terminals",
		User: "4", Assistant: "2", Tool: "6", ToolResult: "8", Error: "1", Accent: "6", Input: "8",
	},
	{
		Name: "light", Desc: "ANSI colors tuned for light terminals",
		User: "4", Assistant: "2", Tool: "6", ToolResult: "8", Error: "1", Accent: "5", Input: "8",
	},
	{
		Name: "dracula", Desc: "Dracula-inspired purples and pinks",
		User: "253", Assistant: "120", Tool: "117", ToolResult: "103", Error: "203", Accent: "141", Input: "61",
	},
	{
		Name: "nord", Desc: "Nord frost blues and muted grays",
		User: "188", Assistant: "150", Tool: "109", ToolResult: "97", Error: "131", Accent: "110", Input: "60",
	},
	{
		Name: "gruvbox", Desc: "Gruvbox warm earth tones",
		User: "223", Assistant: "142", Tool: "109", ToolResult: "245", Error: "167", Accent: "172", Input: "239",
	},
}

// Palettes returns every built-in theme in setup order.
func Palettes() []Palette {
	out := make([]Palette, len(palettes))
	copy(out, palettes)
	return out
}

// Named returns the palette for the given theme name; an unknown name falls
// back to the default ("dark") with ok=false.
func Named(name string) (Palette, bool) {
	for _, p := range palettes {
		if p.Name == name {
			return p, true
		}
	}
	return palettes[0], false
}

// Styles bundles the lipgloss styles derived from a palette.
type Styles struct {
	User       lipgloss.Style
	Assistant  lipgloss.Style
	Tool       lipgloss.Style
	ToolResult lipgloss.Style
	Error      lipgloss.Style
	Accent     lipgloss.Style
	Input      lipgloss.Style
}

// StylesFor builds the lipgloss styles for the named theme; unknown names
// use the default palette.
func StylesFor(name string) Styles {
	p, _ := Named(name)
	fg := func(c string) lipgloss.Style { return lipgloss.NewStyle().Foreground(lipgloss.Color(c)) }
	return Styles{
		User:       fg(p.User),
		Assistant:  fg(p.Assistant),
		Tool:       fg(p.Tool),
		ToolResult: fg(p.ToolResult),
		Error:      fg(p.Error),
		Accent:     fg(p.Accent),
		Input:      lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(p.Input)),
	}
}

// Sample renders a small preview block showing every style, used by the
// setup wizard.
func Sample(p Palette) string {
	s := StylesFor(p.Name)
	return fmt.Sprintf(
		"%s\n%s\n%s\n%s\n%s\n%s",
		s.User.Render("you › fix the failing test in auth_test.go"),
		s.Assistant.Render("bot › on it — the assertion expected 200, got 401."),
		s.Tool.Render("⚙ bash: go test ./internal/auth/"),
		s.ToolResult.Render("  ok    internal/auth  0.31s"),
		s.Error.Render("error: build failed: undefined: Token"),
		s.Accent.Render(fmt.Sprintf("theme %q — %s", p.Name, p.Desc)),
	)
}
