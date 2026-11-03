package theme

import (
	"strings"
	"testing"
)

func TestPalettesCoverExpectedThemes(t *testing.T) {
	names := map[string]bool{}
	for _, p := range Palettes() {
		names[p.Name] = true
	}
	for _, want := range []string{"dark", "light", "dracula", "nord", "gruvbox"} {
		if !names[want] {
			t.Errorf("theme %q missing from Palettes()", want)
		}
	}
}

func TestNamedResolvesKnownAndFallsBackForUnknown(t *testing.T) {
	if p, ok := Named("dracula"); !ok || p.Name != "dracula" {
		t.Errorf("Named(dracula) = %q, %v; want dracula, true", p.Name, ok)
	}
	p, ok := Named("no-such")
	if ok {
		t.Error("Named(no-such) should report not-found")
	}
	def, _ := Named("dark")
	if p.Name != def.Name {
		t.Errorf("unknown theme fallback = %q, want %q", p.Name, def.Name)
	}
}

func TestStylesForBuildsEveryRole(t *testing.T) {
	for _, pal := range Palettes() {
		s := StylesFor(pal.Name)
		samples := map[string]string{
			"user":      s.User.Render("x"),
			"assistant": s.Assistant.Render("x"),
			"tool":      s.Tool.Render("x"),
			"result":    s.ToolResult.Render("x"),
			"error":     s.Error.Render("x"),
			"accent":    s.Accent.Render("x"),
			"input":     s.Input.Render("x"),
		}
		for role, rendered := range samples {
			if strings.TrimSpace(rendered) == "" {
				t.Errorf("theme %s: %s style rendered empty", pal.Name, role)
			}
		}
	}
}

func TestSampleRendersAllRoles(t *testing.T) {
	for _, pal := range Palettes() {
		s := Sample(pal)
		for _, want := range []string{"you ›", "bot ›", "theme"} {
			if !strings.Contains(s, want) {
				t.Errorf("Sample(%s) missing %q", pal.Name, want)
			}
		}
	}
}
