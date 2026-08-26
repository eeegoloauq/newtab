package web

import (
	"net/http"

	"github.com/eeegoloauq/newtab/internal/config"
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
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("ok\n"))
	})
	return mux
}

// Render returns the page as a file, for previewing a config without
// running a server.
func Render(c *config.Config) ([]byte, error) { return render(c) }
