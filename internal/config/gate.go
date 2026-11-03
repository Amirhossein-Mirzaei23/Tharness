package config

import (
	"github.com/Amirhossein-Mirzaei23/Tharness/internal/permissions"
)

// BuildGate maps per-tool config policies onto a permissions.Gate. Only
// bash, write, and edit are gated; everything else stays un-gated (unknown
// tools default to allow in the Gate).
func BuildGate(p Permissions) *permissions.Gate {
	return permissions.NewGate(map[string]permissions.Policy{
		"bash":    permissions.Policy(p.Bash),
		"write":   permissions.Policy(p.Write),
		"edit":    permissions.Policy(p.Edit),
		"browser": permissions.Policy(p.Browser),
	})
}
