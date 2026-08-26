package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func load(t *testing.T, body string) (*Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return Load(path)
}

const minimal = `
sections:
  - name: Infra
    links:
      - name: Git
        url: http://198.51.100.12:3000/
`

func TestDefaults(t *testing.T) {
	c, err := load(t, minimal)
	if err != nil {
		t.Fatal(err)
	}
	if c.Title != defaultTitle || c.Listen != defaultListen || c.Search.Engine != defaultEngine {
		t.Fatalf("defaults not applied: %+v", c)
	}
	if c.Sections[0].Style != StyleList {
		t.Fatalf("a section without a style should be a list, got %q", c.Sections[0].Style)
	}
}

func TestRejects(t *testing.T) {
	cases := map[string]struct{ body, want string }{
		"unknown key": {"foo: bar\n" + minimal, "foo"},
		"no sections": {"title: x\n", "no sections"},
		"bad style": {`
sections:
  - name: A
    style: tiles
    links:
      - {name: X, url: https://example.com/}
`, "style"},
		"duplicate link": {`
sections:
  - name: A
    links:
      - {name: X, url: https://example.com/}
      - {name: X, url: https://example.org/}
`, "twice"},
		"relative url": {`
sections:
  - name: A
    links:
      - {name: X, url: /local}
`, "absolute"},
		"javascript url": {`
sections:
  - name: A
    links:
      - {name: X, url: "javascript:alert(1)"}
`, "absolute"},
		"empty section": {`
sections:
  - name: A
    links: []
`, "no links"},
		"engine without query": {"search:\n  engine: https://example.com/\n" + minimal, "%s"},
		"prefix without query": {"search:\n  prefixes:\n    w: https://example.com/\n" + minimal, "%s"},
		"prefix with a space":  {"search:\n  prefixes:\n    \"w x\": https://example.com/?q=%s\n" + minimal, "before a space"},
		"prefix not http":      {"search:\n  prefixes:\n    w: javascript:alert(1)%s\n" + minimal, "http(s)"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := load(t, tc.body)
			if err == nil {
				t.Fatalf("accepted a config that should have been rejected")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}
