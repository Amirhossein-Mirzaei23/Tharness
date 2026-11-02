package tui

import (
	"strings"
)

// toolCardText renders the one-line tool-call card: "⚙ name(args…)".
func toolCardText(name, argsJSON string) string {
	return "⚙ " + name + "(" + compactArgs(argsJSON) + ")"
}

// toolResultText renders the one-line result preview under a tool card.
func toolResultText(name, result string) string {
	return "  ↳ " + name + ": " + firstLine(result)
}

// compactArgs strips JSON punctuation for a compact single-line display.
func compactArgs(argsJSON string) string {
	var b strings.Builder
	for _, r := range argsJSON {
		switch r {
		case '{', '}', '"':
			// drop
		case ',', ':':
			b.WriteRune(r)
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// firstLine returns the first non-empty line of s, truncated for display.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			if len(line) > 120 {
				line = line[:117] + "…"
			}
			return line
		}
	}
	return "(no output)"
}
