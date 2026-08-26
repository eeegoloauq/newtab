package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eeegoloauq/newtab/internal/config"
)

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func testPage(t *testing.T, iconDir string) *config.Config {
	t.Helper()
	return &config.Config{
		Title:   "newtab",
		IconDir: iconDir,
		Search:  config.Search{Engine: "https://example.com/?q=%s"},
		Sections: []config.Section{{
			Name: "Work",
			Links: []config.Link{
				{Name: "Git", URL: "https://git.example.com/"},
			},
		}},
	}
}

// A cached copy of the start page would hide a link the operator just
// added, and this is the first page of every browsing session.
func TestIndexIsHTMLAndUncached(t *testing.T) {
	rec := get(t, New(testPage(t, ""), nil, nil), "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}
	if !strings.Contains(rec.Body.String(), "Git") {
		t.Errorf("page missing the link name:\n%s", rec.Body.String())
	}
}

// These bytes came from someone else's website. An SVG served from our
// origin is a document; nosniff and the sandbox are what stop it running
// here, and a week is how long the browser should stop asking.
// tinyPNG is a 1x1 image: the smallest thing that sniffs as image/png.
var tinyPNG = []byte{
	0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0a, 'I', 'D', 'A', 'T',
	0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05,
	0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00,
	0x00, 0x00, 'I', 'E', 'N', 'D', 0xae, 0x42, 0x60, 0x82,
}

func TestIconIsServedAsUntrustedBytes(t *testing.T) {
	dir := t.TempDir()
	// Real bytes: the handler refuses to serve a file that does not sniff
	// as an image, because a stored HTML error page named .png is how a
	// site's 404 becomes a document on our own origin.
	if err := os.WriteFile(filepath.Join(dir, "git.png"), tinyPNG, 0o600); err != nil {
		t.Fatal(err)
	}
	rec := get(t, New(testPage(t, dir), nil, nil), "/icon/git")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.Bytes())
	}
	if !bytes.Equal(rec.Body.Bytes(), tinyPNG) {
		t.Errorf("body = %q", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", ct)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", rec.Header().Get("X-Content-Type-Options"))
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "sandbox") {
		t.Errorf("CSP = %q, want sandbox", csp)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "max-age=604800") {
		t.Errorf("Cache-Control = %q, want a week's max-age", cc)
	}
}

// The page only ever asks for /icon/{slug}. A path that walks out of
// icon_dir would serve whatever sits next to it, including a file the
// operator never meant to publish.
func TestIconRejectsPathTraversal(t *testing.T) {
	root := t.TempDir()
	iconDir := filepath.Join(root, "icons")
	if err := os.Mkdir(iconDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const bait = "root:x:0:0:this-file-must-not-be-served"
	for _, name := range []string{"passwd", "passwd.png", "secret.ico"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(bait), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// A legitimate icon in the store: a traversal that is only a glob
	// against the wrong directory must still not return this either.
	if err := os.WriteFile(filepath.Join(iconDir, "git.png"), []byte("icon-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := New(testPage(t, iconDir), nil, nil)
	for _, path := range []string{
		"/icon/..%2f..%2fetc%2fpasswd",
		"/icon/../../etc/passwd",
		"/icon/../passwd",
		"/icon/foo/bar",
		"/icon/foo.bar.baz",
		"/icon/..%2fpasswd.png",
	} {
		rec := get(t, h, path)
		// Unclean ".." paths never reach the handler: ServeMux
		// redirects them. Encoded slashes, dots and extra segments
		// 404. Either way the bait must not be in the body.
		if rec.Code != http.StatusNotFound && rec.Code != http.StatusTemporaryRedirect {
			t.Errorf("%s: status %d, want 404 or 307", path, rec.Code)
		}
		if rec.Code == http.StatusOK {
			t.Errorf("%s: served a file", path)
		}
		if strings.Contains(rec.Body.String(), bait) {
			t.Errorf("%s served the bait file:\n%s", path, rec.Body.String())
		}
		if rec.Body.String() == "icon-bytes" {
			t.Errorf("%s walked into icon_dir and returned an unrelated file", path)
		}
	}
}

// An empty icon_dir is a page of globes, not a broken server: the
// operator may not have run `newtab icons` yet, and the links still
// have to be there.
func TestEmptyIconDirStillRendersThePage(t *testing.T) {
	h := New(testPage(t, ""), nil, nil)
	icon := get(t, h, "/icon/git")
	if icon.Code != http.StatusNotFound {
		t.Errorf("/icon/git: status %d, want 404", icon.Code)
	}
	page := get(t, h, "/")
	if page.Code != http.StatusOK {
		t.Fatalf("GET /: status %d", page.Code)
	}
	if !strings.Contains(page.Body.String(), "Git") {
		t.Errorf("page missing the link:\n%s", page.Body.String())
	}
}

func TestHealthzOK(t *testing.T) {
	rec := get(t, New(testPage(t, ""), nil, nil), "/healthz")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != "ok" {
		t.Errorf("body = %q, want ok", rec.Body.String())
	}
}

// 204 is "no icon", which keeps the console quiet without shipping an asset.
func TestFaviconIsSilent(t *testing.T) {
	rec := get(t, New(testPage(t, ""), nil, nil), "/favicon.ico")
	if rec.Code != http.StatusNoContent {
		t.Errorf("status %d, want 204", rec.Code)
	}
}

func TestUnknownPathIs404(t *testing.T) {
	rec := get(t, New(testPage(t, ""), nil, nil), "/nope")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status %d", rec.Code)
	}
}
