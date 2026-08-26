package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCfg(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

const links = `
search:
  engine: https://example.com/search?q=%s
sections:
  - name: Work
    links:
      - name: Git
        url: https://git.example.com/
`

// A typo'd command must not start the server or walk the network; the
// error is the only signal an operator wrapping this in a script has.
func TestUnknownCommandAndValidateNeedsAFile(t *testing.T) {
	if err := run([]string{"nonsense"}); err == nil {
		t.Error("an unknown command must be an error")
	} else if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("error = %q, want it to name the command", err)
	}
	if err := run([]string{"validate"}); err == nil {
		t.Error("validate without a file must be an error")
	} else if !strings.Contains(err.Error(), "config file") {
		t.Errorf("error = %q, want it to ask for the file", err)
	}
}

// Fetching with nowhere to put the files would hit every site in the
// config and throw the bytes away.
func TestIconsRequiresIconDir(t *testing.T) {
	path := writeCfg(t, t.TempDir(), links)
	err := run([]string{"icons", path})
	if err == nil {
		t.Fatal("icons without icon_dir must be an error")
	}
	if !strings.Contains(err.Error(), "icon_dir") {
		t.Errorf("error = %q, want it to name icon_dir", err)
	}
}

// The whole point of -inline is a file that works from disk, inside a
// browser extension, or anywhere the server is not reachable.
func TestRenderInlineEmbedsIcons(t *testing.T) {
	dir := t.TempDir()
	iconDir := filepath.Join(dir, "icons")
	if err := os.Mkdir(iconDir, 0o755); err != nil {
		t.Fatal(err)
	}
	png := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00,
		0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49,
		0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
	if err := os.WriteFile(filepath.Join(iconDir, "git.png"), png, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := writeCfg(t, dir, "icon_dir: "+iconDir+"\n"+links)
	out := filepath.Join(dir, "page.html")
	if err := run([]string{"render", "-inline", cfg, out}); err != nil {
		t.Fatalf("render -inline: %v", err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "data:image") {
		t.Errorf("inline render missing data:image:\n%s", body)
	}
}
