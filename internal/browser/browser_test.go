package browser

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// fakeBrowser installs an executable script that records its argv into the
// returned file path and exits 0.
func fakeBrowser(t *testing.T, name string) (binDir, logPath string) {
	t.Helper()
	binDir = t.TempDir()
	logPath = filepath.Join(t.TempDir(), "argv.log")
	script := "#!/bin/sh\nfor a in \"$@\"; do echo \"$a\" >> " + logPath + "; done\n"
	if err := os.WriteFile(filepath.Join(binDir, name), []byte(script), 0o700); err != nil {
		t.Fatalf("write fake browser: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+t.TempDir())
	return binDir, logPath
}

func TestResolveFindsCandidateOnPath(t *testing.T) {
	binDir, _ := fakeBrowser(t, "firefox")
	p, err := Resolve("")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if p != filepath.Join(binDir, "firefox") {
		t.Errorf("Resolve = %q, want %q", p, filepath.Join(binDir, "firefox"))
	}
}

func TestResolveUsesConfiguredCommand(t *testing.T) {
	binDir, _ := fakeBrowser(t, "chromium")
	p, err := Resolve("chromium")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if p != filepath.Join(binDir, "chromium") {
		t.Errorf("Resolve = %q, want %q", p, filepath.Join(binDir, "chromium"))
	}
}

func TestResolveMissingConfiguredCommandErrors(t *testing.T) {
	fakeBrowser(t, "firefox") // PATH has no opera
	if _, err := Resolve("opera"); err == nil {
		t.Fatal("Resolve(opera) should fail when not on PATH")
	}
}

func TestResolveEmptyPathErrors(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := Resolve(""); err == nil {
		t.Fatal("Resolve with no browser on PATH should fail with a setup hint")
	}
}

func TestCandidatesPrefersBrowserEnv(t *testing.T) {
	binDir, _ := fakeBrowser(t, "firefox")
	// Install a second browser so the env preference is observable.
	if err := os.WriteFile(filepath.Join(binDir, "chromium"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BROWSER", "chromium")
	got := Candidates()
	if len(got) == 0 || filepath.Base(got[0]) != "chromium" {
		t.Errorf("Candidates[0] = %v, want $BROWSER entry first", got)
	}
}

func TestOpenLaunchesBrowserWithURL(t *testing.T) {
	_, logPath := fakeBrowser(t, "firefox")
	if err := Open(context.Background(), "firefox", "https://example.com/docs"); err != nil {
		t.Fatalf("Open: %v", err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("browser did not run: %v", err)
	}
	if string(data) != "https://example.com/docs\n" {
		t.Errorf("browser argv = %q, want the URL", data)
	}
}

func TestOpenAutoDetectsWhenCommandEmpty(t *testing.T) {
	_, logPath := fakeBrowser(t, "google-chrome")
	if err := Open(context.Background(), "", "https://example.com"); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if data, _ := os.ReadFile(logPath); string(data) != "https://example.com\n" {
		t.Errorf("auto-detected browser argv = %q", data)
	}
}

func TestOpenFailsWhenNoBrowser(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := Open(context.Background(), "", "https://example.com"); err == nil {
		t.Fatal("Open without any browser should fail")
	}
}
