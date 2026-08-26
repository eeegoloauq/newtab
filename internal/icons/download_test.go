package icons

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// tinyPNG is a 1×1 PNG. DetectContentType must see a real image; a
// made-up magic number would not prove the sniffer accepted it.
var tinyPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00,
	0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49,
	0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

func pngTagged(tag byte) []byte {
	return append(append([]byte{}, tinyPNG...), tag)
}

func serveIcons(t *testing.T, h http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func fetch(t *testing.T, page string) (Store, string, error) {
	t.Helper()
	store := Store{Dir: t.TempDir()}
	path, err := store.Fetch(context.Background(), "Git", page)
	return store, path, err
}

func writePNG(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "image/png")
	w.Write(body)
}

func htmlPage(links string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<!doctype html><html><head>" + links + "</head></html>"))
	}
}

func mux(page http.HandlerFunc, files map[string]http.HandlerFunc) http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			if h, ok := files[r.URL.Path]; ok {
				h.ServeHTTP(w, r)
				return
			}
			http.NotFound(w, r)
			return
		}
		page.ServeHTTP(w, r)
	})
	return m
}

// A 16px icon is a blurred smudge on a HiDPI screen; when the page
// declares several, the largest is the one worth storing.
func TestFetchPicksTheLargestDeclaredIcon(t *testing.T) {
	small, large := pngTagged(1), pngTagged(2)
	srv := serveIcons(t, mux(
		htmlPage(`<link rel="icon" sizes="16x16" href="/small.png"><link rel="icon" sizes="32x32" href="/large.png">`),
		map[string]http.HandlerFunc{
			"/small.png": func(w http.ResponseWriter, _ *http.Request) { writePNG(w, small) },
			"/large.png": func(w http.ResponseWriter, _ *http.Request) { writePNG(w, large) },
		},
	))
	_, path, err := fetch(t, srv.URL+"/")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, large) {
		t.Errorf("stored the smaller icon instead of the 32px one")
	}
}

// "any" means SVG, which beats every raster size there is.
func TestFetchPrefersSizesAnyOverRaster(t *testing.T) {
	const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"/>`
	srv := serveIcons(t, mux(
		htmlPage(`<link rel="icon" sizes="512x512" href="/big.png"><link rel="icon" sizes="any" href="/icon.svg">`),
		map[string]http.HandlerFunc{
			"/big.png": func(w http.ResponseWriter, _ *http.Request) { writePNG(w, tinyPNG) },
			"/icon.svg": func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "image/svg+xml")
				w.Write([]byte(svg))
			},
		},
	))
	_, path, err := fetch(t, srv.URL+"/")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(path) != ".svg" {
		t.Errorf("stored %s, want the SVG that declared sizes=any", path)
	}
}

// An apple-touch-icon carries no sizes attribute more often than not,
// and is 180px by convention — enough to beat a 16px favicon and not
// a 192px one.
func TestFetchTreatsAppleTouchIconAs180(t *testing.T) {
	apple, tiny, hidpi := pngTagged(1), pngTagged(2), pngTagged(3)

	t.Run("beats a 16px icon", func(t *testing.T) {
		srv := serveIcons(t, mux(
			htmlPage(`<link rel="icon" sizes="16x16" href="/tiny.png"><link rel="apple-touch-icon" href="/apple.png">`),
			map[string]http.HandlerFunc{
				"/tiny.png":  func(w http.ResponseWriter, _ *http.Request) { writePNG(w, tiny) },
				"/apple.png": func(w http.ResponseWriter, _ *http.Request) { writePNG(w, apple) },
			},
		))
		_, path, err := fetch(t, srv.URL+"/")
		if err != nil {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(path)
		if !bytes.Equal(got, apple) {
			t.Error("stored the 16px icon instead of the apple-touch-icon")
		}
	})

	t.Run("loses to a 192px icon", func(t *testing.T) {
		srv := serveIcons(t, mux(
			htmlPage(`<link rel="icon" sizes="192x192" href="/hidpi.png"><link rel="apple-touch-icon" href="/apple.png">`),
			map[string]http.HandlerFunc{
				"/hidpi.png": func(w http.ResponseWriter, _ *http.Request) { writePNG(w, hidpi) },
				"/apple.png": func(w http.ResponseWriter, _ *http.Request) { writePNG(w, apple) },
			},
		))
		_, path, err := fetch(t, srv.URL+"/")
		if err != nil {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(path)
		if !bytes.Equal(got, hidpi) {
			t.Error("a 192px icon must beat the implicit 180 of apple-touch-icon")
		}
	})
}

// Serving an HTML error page with 200 and an image content type is a
// common way for a site to "have" a favicon. The headers have already
// lied; the next candidate is the one that is actually a picture.
func TestFetchSkipsABodyThatSniffsAsHTML(t *testing.T) {
	real := pngTagged(9)
	srv := serveIcons(t, mux(
		htmlPage(`<link rel="icon" sizes="32x32" href="/fake.png"><link rel="icon" sizes="16x16" href="/real.png">`),
		map[string]http.HandlerFunc{
			"/fake.png": func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "image/png")
				w.Write([]byte("<!doctype html><html><body>not an icon</body></html>"))
			},
			"/real.png": func(w http.ResponseWriter, _ *http.Request) { writePNG(w, real) },
		},
	))
	_, path, err := fetch(t, srv.URL+"/")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, real) {
		t.Error("kept the HTML that claimed to be a PNG")
	}
}

// A favicon above the cap is not a favicon, it is a mistake on the
// other end; an empty body is the same as no icon at all.
func TestFetchRejectsEmptyAndOversizedBodies(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		real := pngTagged(4)
		srv := serveIcons(t, mux(
			htmlPage(`<link rel="icon" sizes="32x32" href="/empty.png"><link rel="icon" sizes="16x16" href="/real.png">`),
			map[string]http.HandlerFunc{
				"/empty.png": func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "image/png")
				},
				"/real.png": func(w http.ResponseWriter, _ *http.Request) { writePNG(w, real) },
			},
		))
		_, path, err := fetch(t, srv.URL+"/")
		if err != nil {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(path)
		if !bytes.Equal(got, real) {
			t.Error("an empty body was stored as the icon")
		}
	})

	t.Run("over the cap", func(t *testing.T) {
		real := pngTagged(5)
		tooBig := append(append([]byte{}, tinyPNG...), bytes.Repeat([]byte{0x00}, maxIcon)...)
		srv := serveIcons(t, mux(
			htmlPage(`<link rel="icon" sizes="32x32" href="/huge.png"><link rel="icon" sizes="16x16" href="/real.png">`),
			map[string]http.HandlerFunc{
				"/huge.png": func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "image/png")
					w.Write(tooBig)
				},
				"/real.png": func(w http.ResponseWriter, _ *http.Request) { writePNG(w, real) },
			},
		))
		_, path, err := fetch(t, srv.URL+"/")
		if err != nil {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(path)
		if !bytes.Equal(got, real) {
			t.Error("a body over 256KB was stored as the icon")
		}
		if info, err := os.Stat(path); err == nil && info.Size() > maxIcon {
			t.Errorf("stored %d bytes, over the cap", info.Size())
		}
	})
}

// A data: URI is the icon, already delivered with the page. Nothing to
// fetch — and no reason to skip a site that inlines its icon, unless
// what it inlined is not an image.
func TestFetchAcceptsImageDataURIsAndRejectsHTML(t *testing.T) {
	t.Run("base64 png", func(t *testing.T) {
		href := "data:image/png;base64," + base64.StdEncoding.EncodeToString(tinyPNG)
		srv := serveIcons(t, mux(htmlPage(`<link rel="icon" href="`+href+`">`), nil))
		_, path, err := fetch(t, srv.URL+"/")
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, tinyPNG) {
			t.Error("did not store the inlined PNG")
		}
		if filepath.Ext(path) != ".png" {
			t.Errorf("stored as %s, want .png", path)
		}
	})

	t.Run("unencoded svg", func(t *testing.T) {
		srv := serveIcons(t, mux(
			htmlPage(`<link rel="icon" href="data:image/svg+xml,<svg xmlns=%22http://www.w3.org/2000/svg%22></svg>">`),
			nil,
		))
		_, path, err := fetch(t, srv.URL+"/")
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(got, []byte("<svg")) {
			t.Errorf("stored %q, want the inlined SVG", got)
		}
		if filepath.Ext(path) != ".svg" {
			t.Errorf("stored as %s, want .svg", path)
		}
	})

	t.Run("html is not an icon", func(t *testing.T) {
		srv := serveIcons(t, mux(
			htmlPage(`<link rel="icon" href="data:text/html,<html>nope</html>">`),
			nil,
		))
		store, _, err := fetch(t, srv.URL+"/")
		if err == nil {
			t.Fatal("an HTML data: URI must not count as an icon")
		}
		if store.Valid("Git") {
			t.Error("stored a data:text/html payload as an icon")
		}
	})
}

// The page refused us; the well-known paths are often still served, and
// that is the difference between "no icon" and a torn square of an
// error page.
func TestFetchFallsBackToFaviconWhenThePageIsDown(t *testing.T) {
	t.Run("500", func(t *testing.T) {
		srv := serveIcons(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/favicon.ico" {
				w.Header().Set("Content-Type", "image/x-icon")
				w.Write(tinyPNG)
				return
			}
			http.Error(w, "nope", http.StatusInternalServerError)
		}))
		_, path, err := fetch(t, srv.URL+"/")
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, tinyPNG) {
			t.Error("did not fall back to /favicon.ico after a 500")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		srv := serveIcons(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/favicon.ico" {
				w.Header().Set("Content-Type", "image/x-icon")
				w.Write(tinyPNG)
				return
			}
			// The client gives up; staying on the request context is
			// what lets the test finish when it does.
			<-r.Context().Done()
		}))
		// A short client timeout keeps this honest and fast: the real
		// fifteen seconds would be thirty in this file alone.
		store := Store{Dir: t.TempDir(), Timeout: 100 * time.Millisecond}
		path, err := store.Fetch(context.Background(), "Git", srv.URL+"/")
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, tinyPNG) {
			t.Error("did not fall back to /favicon.ico after the page timed out")
		}
	})
}

// A file written before the fetcher learned to sniff can be an HTML
// error page with an .ico name, and it renders as a torn square rather
// than as nothing — worse than having no icon at all.
func TestStoreValidRejectsHTMLAndMissingFiles(t *testing.T) {
	dir := t.TempDir()
	store := Store{Dir: dir}

	if store.Valid("Git") {
		t.Error("a missing file must not count as a valid icon")
	}

	if err := os.WriteFile(filepath.Join(dir, "git.ico"), []byte("<!doctype html><html>error</html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if store.Valid("Git") {
		t.Error("HTML stored as .ico must not count as a valid icon")
	}

	if err := os.WriteFile(filepath.Join(dir, "git.png"), tinyPNG, 0o600); err != nil {
		t.Fatal(err)
	}
	// Path globs git.*; with both files present the first match might
	// still be the HTML. Drop it so the PNG is what Valid sees.
	if err := os.Remove(filepath.Join(dir, "git.ico")); err != nil {
		t.Fatal(err)
	}
	if !store.Valid("Git") {
		t.Error("a PNG must count as a valid icon")
	}
}

// Write first, delete after. Deleting first meant a flaky network on a
// -force run wiped a working icon and put nothing back; a leftover
// *.part is the same as a half-written file the page might serve.
func TestFetchReplacesTheOldFileAndLeavesNoPartFiles(t *testing.T) {
	srv := serveIcons(t, mux(
		htmlPage(`<link rel="icon" href="/icon.png">`),
		map[string]http.HandlerFunc{
			"/icon.png": func(w http.ResponseWriter, _ *http.Request) { writePNG(w, tinyPNG) },
		},
	))
	dir := t.TempDir()
	old := filepath.Join(dir, "git.ico")
	if err := os.WriteFile(old, []byte("old-ico"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := Store{Dir: dir}
	path, err := store.Fetch(context.Background(), "Git", srv.URL+"/")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("old .ico still there after a .png was stored: %v", err)
	}
	if filepath.Ext(path) == ".ico" {
		t.Error("kept the old extension")
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*.part"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Errorf("leftover part files: %v", matches)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, tinyPNG) {
		t.Error("the new file is not the PNG we fetched")
	}
}

func TestFetchUsesABoundedClientTimeout(t *testing.T) {
	// Guards the fallback test above: if the client ever lost its
	// timeout, that hang would be the whole test run.
	if newClient(0).Timeout != defaultTimeout {
		t.Fatalf("client timeout = %s", newClient(0).Timeout)
	}
	if newClient(time.Second).Timeout != time.Second {
		t.Fatal("an explicit timeout must win")
	}
}
