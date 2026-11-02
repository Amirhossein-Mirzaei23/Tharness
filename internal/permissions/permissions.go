// Package permissions gates tool execution behind per-tool policies
// ("auto", "ask", "deny").
package permissions

import (
	"context"
	"fmt"
	"sync"

	"github.com/amir/tabdl/internal/tools"
)

// Policy governs how a tool call is approved.
type Policy string

const (
	// PolicyAuto executes without prompting (the default).
	PolicyAuto Policy = "auto"
	// PolicyAsk routes each call through an interactive Asker.
	PolicyAsk Policy = "ask"
	// PolicyDeny rejects every call without executing the tool.
	PolicyDeny Policy = "deny"
)

// Decision is the outcome of Gate.Check.
type Decision string

const (
	// DecisionAllow executes the tool without prompting.
	DecisionAllow Decision = "allow"
	// DecisionDeny rejects the call.
	DecisionDeny Decision = "deny"
	// DecisionAsk requires interactive approval.
	DecisionAsk Decision = "ask"
)

// Gate resolves per-tool policies for the session. Unknown tools default to
// PolicyAuto (allow): only explicitly configured tools are gated.
type Gate struct {
	mu             sync.Mutex
	policies       map[string]Policy
	sessionAllowed map[string]bool
}

// NewGate returns a Gate over the given per-tool policies (nil map = all auto).
func NewGate(policies map[string]Policy) *Gate {
	if policies == nil {
		policies = map[string]Policy{}
	}
	return &Gate{
		policies:       policies,
		sessionAllowed: map[string]bool{},
	}
}

// Check resolves the decision for one tool name.
func (g *Gate) Check(tool string) Decision {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.sessionAllowed[tool] {
		return DecisionAllow
	}
	p, ok := g.policies[tool]
	if !ok {
		return DecisionAllow // unknown tools are open by default (documented)
	}
	switch p {
	case PolicyDeny:
		return DecisionDeny
	case PolicyAsk:
		return DecisionAsk
	default:
		return DecisionAllow
	}
}

// AllowSession records that the user approved tool for the rest of the
// session ("always allow" in the TUI); subsequent Checks return allow.
func (g *Gate) AllowSession(tool string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.sessionAllowed[tool] = true
}

// Asker decides one pending "ask" approval. It receives the tool name and a
// compact summary of the arguments, and returns true to allow the call.
type Asker func(tool, argsSummary string) bool

// GatedTool wraps a tools.Tool and consults a Gate before executing.
type GatedTool struct {
	inner tools.Tool
	gate  *Gate
	ask   Asker
}

// NewGatedTool wraps inner with the given Gate. The Asker is consulted only
// for tools whose policy is "ask"; a nil Asker with an "ask" policy denies
// the call (non-interactive runs cannot prompt).
func NewGatedTool(inner tools.Tool, gate *Gate, ask Asker) *GatedTool {
	return &GatedTool{inner: inner, gate: gate, ask: ask}
}

func (g *GatedTool) Name() string               { return g.inner.Name() }
func (g *GatedTool) Description() string        { return g.inner.Description() }
func (g *GatedTool) Parameters() map[string]any { return g.inner.Parameters() }

// Exec consults the Gate, then delegates to the wrapped tool.
func (g *GatedTool) Exec(ctx context.Context, argsJSON string) (string, error) {
	switch g.gate.Check(g.inner.Name()) {
	case DecisionAllow:
		return g.inner.Exec(ctx, argsJSON)
	case DecisionDeny:
		return "", fmt.Errorf("permission denied by policy (%s)", g.inner.Name())
	case DecisionAsk:
		if g.ask == nil {
			return "", fmt.Errorf("permission denied: %s requires approval and this run is non-interactive", g.inner.Name())
		}
		if g.ask(g.inner.Name(), argsJSON) {
			return g.inner.Exec(ctx, argsJSON)
		}
		return "", fmt.Errorf("user denied %s", g.inner.Name())
	default:
		return "", fmt.Errorf("permission denied by policy (%s)", g.inner.Name())
	}
}

// gatedTools are the tools subject to permission gating (Task 13).
var gatedTools = []string{"bash", "write", "edit"}

// GateRegistry replaces the named gated tools in reg with permission-checked
// wrappers consulting gate. The Asker is consulted only for "ask" policies
// (nil Asker = non-interactive: "ask" falls back to deny). Tools not named in
// gatedTools are left untouched.
func GateRegistry(reg *tools.Registry, gate *Gate, ask Asker) {
	for _, name := range gatedTools {
		inner, ok := reg.Get(name)
		if !ok {
			continue
		}
		reg.Register(NewGatedTool(inner, gate, ask))
	}
}
