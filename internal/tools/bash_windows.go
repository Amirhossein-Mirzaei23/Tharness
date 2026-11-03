//go:build windows

package tools

import (
	"context"
	"os/exec"
)

// newBashCmd runs command through the Windows shell (cmd /c). The process
// runs without a dedicated process group; killBashTree kills the shell only.
func newBashCmd(ctx context.Context, command string) *exec.Cmd {
	return exec.CommandContext(ctx, "cmd", "/c", command)
}

// killBashTree kills the shell process. Grandchildren spawned by the shell
// are not tracked on Windows (Job Objects would be required).
func killBashTree(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
