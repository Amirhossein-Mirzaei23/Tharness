package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Amirhossein-Mirzaei23/Tharness/internal/browser"
)

// BrowserTool opens URLs in the browser configured by `tharness setup`.
type BrowserTool struct {
	// command is the configured browser executable; empty auto-detects.
	command string
}

// NewBrowserTool returns the browser tool using the given command (empty =
// auto-detect at exec time).
func NewBrowserTool(command string) *BrowserTool {
	return &BrowserTool{command: command}
}

func (t *BrowserTool) Name() string { return "browser" }

func (t *BrowserTool) Description() string {
	return "Open a URL in the user's web browser. Use for documentation, dashboards, " +
		"or pages the user asked to view; it cannot read page content back."
}

func (t *BrowserTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "The http(s) URL to open.",
			},
		},
		"required": []string{"url"},
	}
}

// browserArgs is the JSON argument shape.
type browserArgs struct {
	URL string `json:"url"`
}

// Exec opens the URL with the configured browser.
func (t *BrowserTool) Exec(ctx context.Context, argsJSON string) (string, error) {
	var args browserArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("browser: parse args: %w", err)
	}
	url := strings.TrimSpace(args.URL)
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return "", fmt.Errorf("browser: url must start with http:// or https://, got %q", url)
	}
	if err := browser.Open(ctx, t.command, url); err != nil {
		return "", err
	}
	return fmt.Sprintf("opened %s", url), nil
}
