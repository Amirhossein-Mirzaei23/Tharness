package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrowserToolRegisteredInCoreRegistry(t *testing.T) {
	reg := NewCoreRegistry(t.TempDir())
	if _, ok := reg.Get("browser"); !ok {
		t.Fatal("browser tool missing from the core registry")
	}
}

func TestBrowserToolShape(t *testing.T) {
	tool := NewBrowserTool("")
	if tool.Name() != "browser" {
		t.Errorf("Name = %q, want browser", tool.Name())
	}
	url, ok := tool.Parameters()["properties"].(map[string]any)["url"]
	if !ok {
		t.Fatal("parameters must define url")
	}
	urlMap := url.(map[string]any)
	if urlMap["type"] != "string" {
		t.Errorf("url type = %v, want string", urlMap["type"])
	}
}

func TestBrowserToolRejectsNonHTTPURL(t *testing.T) {
	tool := NewBrowserTool("does-not-matter")
	for _, bad := range []string{"file:///etc/passwd", "javascript:alert(1)", "example.com", ""} {
		args, _ := json.Marshal(map[string]string{"url": bad})
		if _, err := tool.Exec(context.Background(), string(args)); err == nil {
			t.Errorf("Exec(url=%q) should reject non-http(s) URLs", bad)
		}
	}
}

func TestBrowserToolOpensWithConfiguredCommand(t *testing.T) {
	binDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "argv.log")
	script := "#!/bin/sh\nfor a in \"$@\"; do echo \"$a\" >> " + logPath + "; done\n"
	if err := os.WriteFile(filepath.Join(binDir, "test-browser"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}

	tool := NewBrowserTool(filepath.Join(binDir, "test-browser"))
	args, _ := json.Marshal(map[string]string{"url": "https://example.com/x"})
	out, err := tool.Exec(context.Background(), string(args))
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if !strings.Contains(out, "https://example.com/x") {
		t.Errorf("Exec result = %q, want URL confirmation", out)
	}
	if data, _ := os.ReadFile(logPath); string(data) != "https://example.com/x\n" {
		t.Errorf("browser argv = %q, want the URL", data)
	}
}
