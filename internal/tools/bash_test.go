package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBashEchoCapturesOutput(t *testing.T) {
	ws := newTestWorkspace(t)
	tool := NewBashTool(ws, BashOptions{})

	out, err := tool.Exec(context.Background(), `{"command":"echo hello-bash"}`)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if !strings.Contains(out, "hello-bash") {
		t.Fatalf("output should contain hello-bash, got: %q", out)
	}
	if strings.Contains(out, "exit status") {
		t.Fatalf("exit 0 should not add status line, got: %q", out)
	}
}

func TestBashNonZeroExitIsDataNotError(t *testing.T) {
	ws := newTestWorkspace(t)
	tool := NewBashTool(ws, BashOptions{})

	out, err := tool.Exec(context.Background(), `{"command":"echo oops >&2; exit 3"}`)
	if err != nil {
		t.Fatalf("non-zero exit must not be a tool error, got: %v", err)
	}
	if !strings.Contains(out, "exit status 3") {
		t.Fatalf("output should contain exit status 3, got: %q", out)
	}
	if !strings.Contains(out, "oops") {
		t.Fatalf("stderr text should be captured, got: %q", out)
	}
}

func TestBashCwdPersistsAcrossCalls(t *testing.T) {
	ws := newTestWorkspace(t)
	if err := os.MkdirAll(filepath.Join(ws, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	tool := NewBashTool(ws, BashOptions{})

	out, err := tool.Exec(context.Background(), `{"command":"pwd","cwd":"sub"}`)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if !strings.HasSuffix(strings.TrimSpace(out), "/sub") {
		t.Fatalf("first call should run in sub, got: %q", out)
	}

	out, err = tool.Exec(context.Background(), `{"command":"pwd"}`)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if !strings.HasSuffix(strings.TrimSpace(out), "/sub") {
		t.Fatalf("second call should still be in sub, got: %q", out)
	}
}

func TestBashCwdEscapeRefused(t *testing.T) {
	ws := newTestWorkspace(t)
	tool := NewBashTool(ws, BashOptions{})

	_, err := tool.Exec(context.Background(), `{"command":"pwd","cwd":"../.."}`)
	if err == nil {
		t.Fatal("expected error for cwd outside workspace")
	}
	if !strings.Contains(err.Error(), "outside the workspace") {
		t.Fatalf("error should mention outside workspace, got: %v", err)
	}

	// cwd must be unchanged after a refused escape: still at workspace root.
	out, err := tool.Exec(context.Background(), `{"command":"pwd"}`)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if strings.TrimSpace(out) != ws {
		t.Fatalf("cwd should still be workspace root %s, got %q", ws, out)
	}
}

func TestBashTimeoutKillsAndReturns(t *testing.T) {
	ws := newTestWorkspace(t)
	tool := NewBashTool(ws, BashOptions{})

	start := time.Now()
	out, err := tool.Exec(context.Background(), `{"command":"echo partial; sleep 10","timeout_seconds":1}`)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("timeout should be data, not tool error: %v", err)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("should return promptly on timeout, took %v", elapsed)
	}
	if !strings.Contains(out, "timed out") {
		t.Fatalf("output should mention timeout, got: %q", out)
	}
	if !strings.Contains(out, "partial") {
		t.Fatalf("partial output should be preserved, got: %q", out)
	}
}

func TestBashTruncatesLongOutput(t *testing.T) {
	ws := newTestWorkspace(t)
	tool := NewBashTool(ws, BashOptions{})

	out, err := tool.Exec(context.Background(), `{"command":"yes abcd | head -c 5000"}`)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if len(out) > 2200 {
		t.Fatalf("output should be capped near 2000 chars, got %d", len(out))
	}
	if !strings.Contains(out, "truncated") {
		t.Fatalf("output should carry a truncation note, got tail: %q", out[len(out)-100:])
	}
}

func TestBashRegisteredInCoreRegistry(t *testing.T) {
	ws := newTestWorkspace(t)
	reg := NewCoreRegistry(ws)
	if _, ok := reg.Get("bash"); !ok {
		t.Fatal("bash should be registered by NewCoreRegistry")
	}
}
