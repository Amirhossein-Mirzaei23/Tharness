// Package browser detects installed browsers and opens URLs with the
// browser configured by `tharness setup`.
package browser

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// candidateNames lists browser executables in auto-detect preference order.
var candidateNames = []string{
	"google-chrome-stable",
	"google-chrome",
	"chromium",
	"chromium-browser",
	"brave-browser",
	"microsoft-edge-stable",
	"microsoft-edge",
	"firefox",
	"opera",
}

// platformCandidates returns OS-specific default-browser launchers. They are
// probed after the explicit browser names: macOS `open` and Windows
// `rundll32` always exist and use the user's default browser.
func platformCandidates() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"open"}
	case "windows":
		return []string{"rundll32"}
	}
	return nil
}

// Candidates returns the browsable executables found on this machine, in
// preference order: entries of $BROWSER first, then the built-in candidate
// list filtered by PATH lookup. The result may be empty.
func Candidates() []string {
	var found []string
	seen := map[string]bool{}
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		if p, err := exec.LookPath(name); err == nil {
			found = append(found, p)
		}
	}
	for _, name := range strings.Fields(os.Getenv("BROWSER")) {
		add(name)
	}
	for _, name := range candidateNames {
		add(name)
	}
	for _, name := range platformCandidates() {
		add(name)
	}
	return found
}

// Resolve maps a configured browser entry (name or absolute path) to an
// executable path. An empty configured value auto-detects; nothing found is
// an error directing the user to setup.
func Resolve(configured string) (string, error) {
	if configured != "" {
		if p, err := exec.LookPath(configured); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("browser: %q not found on PATH (run `tharness setup` to reconfigure)", configured)
	}
	if c := Candidates(); len(c) > 0 {
		return c[0], nil
	}
	return "", fmt.Errorf("browser: no browser found (run `tharness setup` to configure one)")
}

// newBrowserCmd builds the launch command for one URL. On Windows the
// auto-detected rundll32 launcher needs its protocol-handler subcommand.
func newBrowserCmd(ctx context.Context, command, url string) *exec.Cmd {
	if runtime.GOOS == "windows" && strings.EqualFold(filepath.Base(command), "rundll32.exe") {
		return exec.CommandContext(ctx, command, "url.dll,FileProtocolHandler", url)
	}
	return exec.CommandContext(ctx, command, url)
}

// Open opens url with the given browser executable, waiting briefly for the
// process to hand off. A non-zero exit after startup surfaces stderr.
func Open(ctx context.Context, command, url string) error {
	if command == "" {
		resolved, err := Resolve("")
		if err != nil {
			return err
		}
		command = resolved
	}
	cmd := newBrowserCmd(ctx, command, url)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("browser: start %s: %w", command, err)
	}
	// Browsers typically fork and detach; only report errors from the
	// initial hand-off window.
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
				return fmt.Errorf("browser: %s exited with %v", command, err)
			}
		}
	case <-ctx.Done():
	}
	return nil
}
