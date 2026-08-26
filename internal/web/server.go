package web

import (
	"net/http"
	"os"

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
		// Slug is idempotent, so a slug from the URL names the same file
		// as a slug from a link name — and nothing else: the path
		// separator does not survive it.
		path := icons.Store{Dir: dir}.IconBySlug(slug)
		if path == "" {
			http.NotFound(w, r)
			return
		}
		body, err := os.ReadFile(path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", icons.ContentType(path))
		// These bytes came from someone else's website. An SVG served
		// from our own origin is a document, and opening /icon/x
		// directly would run any script inside it here. The sandbox and
		// the null default-src make that inert; nosniff stops a PNG that
		// is really HTML from being treated as one.
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
		// The icon for a link changes when the site changes it and we
		// re-fetch, which is a manual act. Until then it is the same
		// bytes forever, and the browser should stop asking.
		w.Header().Set("Cache-Control", "public, max-age=604800")
		w.Write(body)
	}
}

// Render returns the page as a file. With inline set, the icons are
// embedded in it and the result needs nothing else — no server, no
// network, no icon directory.
func Render(c *config.Config, inline bool) ([]byte, error) {
	if inline {
		return buildInline(c)
	}
	return render(c)
}
