package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	// bashDefaultTimeout is the per-command timeout when the caller does not
	// pass timeout_seconds.
	bashDefaultTimeout = 30 * time.Second
	// bashMaxTimeout caps timeout_seconds.
	bashMaxTimeout = 300 * time.Second
	// bashMaxOutput is the output truncation limit. HEAD of the output is
	// kept (plus a truncation note) — the leading output (errors, early
	// progress) is usually most relevant to the model.
	bashMaxOutput = 2000
)

// BashTool runs shell commands via `sh -c`, capturing combined stdout+stderr.
// Non-zero exits are returned as data (prefixed with an "exit status N" line),
// not as tool errors.
//
// The tool holds a persistent current working directory across Exec calls,
// starting at the workspace root. A {"cwd": "<relative-path>"} argument moves
// it for subsequent calls. Convention: the cwd is only advanced on command
// success (exit 0), matching typical agent-harness semantics — a failing
// command must not leave the session in a directory the model did not verify.
//
// Each command runs under the platform shell (sh on Unix, cmd on Windows).
// On Unix it gets its own process group (Setpgid); on timeout or context
// cancellation the whole group is killed (negative-pid syscall.Kill) so
// spawned children die too. On Windows only the shell process is killed.
type BashTool struct {
	root string
	opts BashOptions
	cwd  string // current working dir for the next call; always inside root
}

// BashOptions configures the bash tool.
type BashOptions struct{}

// NewBashTool returns a BashTool rooted at the absolute workspace dir.
func NewBashTool(root string, opts BashOptions) *BashTool {
	return &BashTool{root: root, opts: opts, cwd: root}
}

func (t *BashTool) Name() string { return "bash" }

func (t *BashTool) Description() string {
	return "Run a shell command in the workspace. Captures stdout and stderr."
}

func (t *BashTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command":         map[string]any{"type": "string", "description": "Shell command to run"},
			"cwd":             map[string]any{"type": "string", "description": "Working directory relative to the workspace root; persists for subsequent calls"},
			"timeout_seconds": map[string]any{"type": "integer", "description": "Timeout in seconds (default 30, max 300)"},
		},
		"required": []string{"command"},
	}
}

func (t *BashTool) Exec(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Command        string `json:"command"`
		Cwd            string `json:"cwd"`
		TimeoutSeconds int    `json:"timeout_seconds"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("bash: parse args: %w", err)
	}
	if args.Command == "" {
		return "", fmt.Errorf("bash: command is required")
	}
	timeout := time.Duration(args.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = bashDefaultTimeout
	}
	if timeout > bashMaxTimeout {
		timeout = bashMaxTimeout
	}
	dir := t.cwd
	if args.Cwd != "" {
		// Workspace guard: only local (non-escaping, relative) paths allowed.
		if !filepath.IsLocal(args.Cwd) {
			return "", fmt.Errorf("bash: cwd %q is outside the workspace", args.Cwd)
		}
		dir = filepath.Join(t.root, filepath.Clean(args.Cwd))
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := newBashCmd(ctx, args.Command)
	cmd.Dir = dir
	// The command's children inherit our output pipe; after the group leader
	// dies, Wait would block on the pipe until every orphan exits. WaitDelay
	// bounds that wait so we return promptly and can then kill the group.
	cmd.WaitDelay = time.Second
	var combined strings.Builder
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	runErr := cmd.Run()

	var out string
	switch {
	case ctx.Err() != nil:
		// Timeout or cancellation: kill the whole process tree.
		killBashTree(cmd)
		out = fmt.Sprintf("timed out after %s\n%s", timeout, combined.String())
	case runErr != nil:
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			out = fmt.Sprintf("exit status %d\n%s", exitErr.ExitCode(), combined.String())
		} else {
			return "", fmt.Errorf("bash: %w", runErr)
		}
	default:
		out = combined.String()
	}
	// Persist cwd only on success (exit 0, no timeout).
	if args.Cwd != "" && runErr == nil && ctx.Err() == nil {
		t.cwd = dir
	}
	return truncateBashOutput(out), nil
}

// truncateBashOutput caps output at bashMaxOutput chars, keeping the head and
// appending a truncation note (see bashMaxOutput comment for the rationale).
func truncateBashOutput(s string) string {
	if len(s) <= bashMaxOutput {
		return s
	}
	return fmt.Sprintf("%s\n[output truncated at %d characters]", s[:bashMaxOutput], bashMaxOutput)
}
