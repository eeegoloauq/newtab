package web

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/eeegoloauq/newtab/internal/config"
	"github.com/eeegoloauq/newtab/internal/icons"
)

// New returns the handler for every endpoint newtab serves.
func New(c *config.Config) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		body, err := render(c)
		if err != nil {
			http.Error(w, "render failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// This is the first page of every browsing session; a stale copy
		// would hide a link the operator just added.
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(body)
	})
	// Asked for on every visit. 204 is "no icon", which keeps the console
	// quiet without shipping an asset.
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /icon/{slug}", iconHandler(c.IconDir))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("ok\n"))
	})
	return mux
}

// iconHandler serves one stored icon. The page only links to icons that
// existed at render time, so a miss here means the store changed under a
// stale tab, not a broken page.
func iconHandler(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := icons.Slug(r.PathValue("slug"))
		if dir == "" || slug == "" {
			http.NotFound(w, r)
			return
		}
		matches, err := filepath.Glob(filepath.Join(dir, slug+".*"))
		if err != nil || len(matches) == 0 {
			http.NotFound(w, r)
			return
		}
		body, err := os.ReadFile(matches[0])
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", icons.ContentType(matches[0]))
		// The icon for a link changes when the site changes it and we
		// re-fetch, which is a manual act. Until then it is the same
		// bytes forever, and the browser should stop asking.
		w.Header().Set("Cache-Control", "public, max-age=604800")
		w.Write(body)
	}
}

// Render returns the page as a file, for previewing a config without
// running a server.
func Render(c *config.Config) ([]byte, error) { return render(c) }
