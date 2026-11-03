//go:build unix

package tools

import (
	"context"
	"os/exec"
	"syscall"
)

// newBashCmd starts command under sh with its own process group so
// timeout/cancel can kill the whole tree.
func newBashCmd(ctx context.Context, command string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

// killBashTree kills the command's process group (negative pid).
func killBashTree(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
