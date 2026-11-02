package permissions

import (
	"context"
	"strings"
	"testing"
)

// RED: policy "auto" allows the tool.
func TestGateCheckAutoAllows(t *testing.T) {
	g := NewGate(map[string]Policy{"bash": PolicyAuto})
	if got := g.Check("bash"); got != DecisionAllow {
		t.Errorf("Check(bash) = %v, want %v", got, DecisionAllow)
	}
}

// RED: unknown tools default to allow (only configured tools are gated).
func TestGateCheckUnknownToolDefaultsToAllow(t *testing.T) {
	g := NewGate(nil)
	if got := g.Check("grep"); got != DecisionAllow {
		t.Errorf("Check(grep) = %v, want %v", got, DecisionAllow)
	}
}

// RED: policy "deny" denies the tool.
func TestGateCheckDenyDenies(t *testing.T) {
	g := NewGate(map[string]Policy{"bash": PolicyDeny})
	if got := g.Check("bash"); got != DecisionDeny {
		t.Errorf("Check(bash) = %v, want %v", got, DecisionDeny)
	}
}

// RED: policy "ask" requires interactive approval.
func TestGateCheckAskAsks(t *testing.T) {
	g := NewGate(map[string]Policy{"write": PolicyAsk})
	if got := g.Check("write"); got != DecisionAsk {
		t.Errorf("Check(write) = %v, want %v", got, DecisionAsk)
	}
}

// RED: AllowSession flips a later Check from ask to allow for the session.
func TestAllowSessionFlipsAskToAllow(t *testing.T) {
	g := NewGate(map[string]Policy{"bash": PolicyAsk})
	if got := g.Check("bash"); got != DecisionAsk {
		t.Fatalf("Check(bash) = %v, want %v before AllowSession", got, DecisionAsk)
	}
	g.AllowSession("bash")
	if got := g.Check("bash"); got != DecisionAllow {
		t.Errorf("Check(bash) after AllowSession = %v, want %v", got, DecisionAllow)
	}
}

// tuiFakeTool is a recording fake for GatedTool wiring tests.
type tuiFakeTool struct {
	calls int
}

func (f *tuiFakeTool) Name() string               { return "bash" }
func (f *tuiFakeTool) Description() string        { return "fake" }
func (f *tuiFakeTool) Parameters() map[string]any { return nil }
func (f *tuiFakeTool) Exec(context.Context, string) (string, error) {
	f.calls++
	return "ran", nil
}

// RED: the approval-channel Asker blocks until the channel receives a
// decision; "y" (allow once) executes the tool.
func TestAskerFromChannelAllowOnce(t *testing.T) {
	gate := NewGate(map[string]Policy{"bash": PolicyAsk})
	approvals := make(chan Approval, 1)
	reg := tools.NewRegistry()
	inner := &tuiFakeTool{}
	reg.Register(NewGatedTool(inner, gate, AskerFromChannel(approvals)))

	done := make(chan struct{})
	var got string
	var err error
	go func() {
		got, err = reg.Exec(context.Background(), "bash", `{}`)
		close(done)
	}()

	select {
	case done <- struct{}{}:
		t.Fatal("Exec returned before a decision was delivered")
	default:
	}

	approvals <- Approval{Decision: DecisionAllow}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Exec did not finish after allow decision")
	}
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if got != "ran" {
		t.Errorf("Exec = %q, want %q", got, "ran")
	}
	if inner.calls != 1 {
		t.Errorf("inner Exec calls = %d, want 1", inner.calls)
	}
}

// RED: a deny decision errors and the underlying tool never runs.
func TestAskerFromChannelDeny(t *testing.T) {
	gate := NewGate(map[string]Policy{"bash": PolicyAsk})
	approvals := make(chan Approval, 1)
	reg := tools.NewRegistry()
	inner := &tuiFakeTool{}
	reg.Register(NewGatedTool(inner, gate, AskerFromChannel(approvals)))

	done := make(chan struct{})
	var err error
	go func() {
		_, err = reg.Exec(context.Background(), "bash", `{}`)
		close(done)
	}()

	approvals <- Approval{Decision: DecisionDeny}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Exec did not finish after deny decision")
	}
	if err == nil || !strings.Contains(err.Error(), "user denied bash") {
		t.Errorf("err = %v, want user denial", err)
	}
	if inner.calls != 0 {
		t.Errorf("inner Exec calls = %d, want 0", inner.calls)
	}
}

// RED: an always-allow decision flips the session policy, so the next call
// runs without waiting on the channel.
func TestAskerFromChannelAlwaysAllow(t *testing.T) {
	gate := NewGate(map[string]Policy{"bash": PolicyAsk})
	approvals := make(chan Approval, 1)
	reg := tools.NewRegistry()
	inner := &tuiFakeTool{}
	reg.Register(NewGatedTool(inner, gate, AskerFromChannel(approvals)))

	done := make(chan struct{})
	go func() {
		_, _ = reg.Exec(context.Background(), "bash", `{}`)
		close(done)
	}()
	approvals <- Approval{Decision: DecisionAllowSession}
	<-done

	// Second call must not need the channel at all.
	got, err := reg.Exec(context.Background(), "bash", `{}`)
	if err != nil {
		t.Fatalf("second Exec returned error: %v", err)
	}
	if got != "ran" {
		t.Errorf("second Exec = %q, want %q", got, "ran")
	}
	if inner.calls != 2 {
		t.Errorf("inner Exec calls = %d, want 2", inner.calls)
	}
}

// fakeTool records whether Exec ran and what it returned.
type fakeTool struct {
	name   string
	calls  int
	result string
	err    error
}

func (f *fakeTool) Name() string                   { return f.name }
func (f *fakeTool) Description() string            { return "fake tool for tests" }
func (f *fakeTool) Parameters() map[string]any     { return nil }
func (f *fakeTool) Exec(context.Context, string) (string, error) {
	f.calls++
	return f.result, f.err
}

// RED: policy "auto" passes the call straight through to the wrapped tool.
func TestGatedToolAutoPassesThrough(t *testing.T) {
	inner := &fakeTool{name: "bash", result: "ok"}
	gt := NewGatedTool(inner, NewGate(map[string]Policy{"bash": PolicyAuto}), nil)

	got, err := gt.Exec(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if got != "ok" {
		t.Errorf("Exec = %q, want %q", got, "ok")
	}
	if inner.calls != 1 {
		t.Errorf("inner Exec calls = %d, want 1", inner.calls)
	}
}

// RED: policy "deny" returns an error and never runs the underlying tool.
func TestGatedToolDenyDoesNotExec(t *testing.T) {
	inner := &fakeTool{name: "bash", result: "SHOULD NOT RUN"}
	gt := NewGatedTool(inner, NewGate(map[string]Policy{"bash": PolicyDeny}), nil)

	got, err := gt.Exec(context.Background(), `{}`)
	if err == nil {
		t.Fatalf("Exec returned nil error, want policy denial")
	}
	if !strings.Contains(err.Error(), "permission denied by policy (bash)") {
		t.Errorf("err = %v, want it to contain %q", err, "permission denied by policy (bash)")
	}
	if got != "" {
		t.Errorf("Exec result = %q, want empty", got)
	}
	if inner.calls != 0 {
		t.Errorf("inner Exec calls = %d, want 0 (denied before exec)", inner.calls)
	}
}

// RED: "ask" + Asker returning false denies without executing.
func TestGatedToolAskDeniedByAsker(t *testing.T) {
	inner := &fakeTool{name: "bash", result: "SHOULD NOT RUN"}
	gt := NewGatedTool(inner, NewGate(map[string]Policy{"bash": PolicyAsk}),
		func(tool, args string) bool { return false })

	_, err := gt.Exec(context.Background(), `{"command":"rm -rf /"}`)
	if err == nil {
		t.Fatalf("Exec returned nil error, want user denial")
	}
	if !strings.Contains(err.Error(), "user denied bash") {
		t.Errorf("err = %v, want it to contain %q", err, "user denied bash")
	}
	if inner.calls != 0 {
		t.Errorf("inner Exec calls = %d, want 0", inner.calls)
	}
}

// RED: "ask" + Asker returning true executes the underlying tool.
func TestGatedToolAskAllowedByAsker(t *testing.T) {
	inner := &fakeTool{name: "bash", result: "ran"}
	var gotTool, gotArgs string
	gt := NewGatedTool(inner, NewGate(map[string]Policy{"bash": PolicyAsk}),
		func(tool, args string) bool {
			gotTool, gotArgs = tool, args
			return true
		})

	got, err := gt.Exec(context.Background(), `{"command":"ls"}`)
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if got != "ran" {
		t.Errorf("Exec = %q, want %q", got, "ran")
	}
	if inner.calls != 1 {
		t.Errorf("inner Exec calls = %d, want 1", inner.calls)
	}
	if gotTool != "bash" || gotArgs != `{"command":"ls"}` {
		t.Errorf("Asker got (%q, %q), want (bash, args JSON)", gotTool, gotArgs)
	}
}

// RED: "ask" with a nil Asker falls back to deny with a clear message
// (non-interactive one-shot runs cannot prompt).
func TestGatedToolAskWithNilAskerFallsBackToDeny(t *testing.T) {
	inner := &fakeTool{name: "bash", result: "SHOULD NOT RUN"}
	gt := NewGatedTool(inner, NewGate(map[string]Policy{"bash": PolicyAsk}), nil)

	_, err := gt.Exec(context.Background(), `{}`)
	if err == nil {
		t.Fatalf("Exec returned nil error, want non-interactive denial")
	}
	if !strings.Contains(err.Error(), "non-interactive") {
		t.Errorf("err = %v, want it to mention non-interactive fallback", err)
	}
	if inner.calls != 0 {
		t.Errorf("inner Exec calls = %d, want 0", inner.calls)
	}
}
