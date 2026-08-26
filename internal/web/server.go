package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/eeegoloauq/newtab/internal/config"
	"github.com/eeegoloauq/newtab/internal/icons"
	"github.com/eeegoloauq/newtab/internal/proxmox"
	"github.com/eeegoloauq/newtab/internal/rates"
	"github.com/eeegoloauq/newtab/internal/status"
	"github.com/eeegoloauq/newtab/internal/weather"
)

// New returns the handler for every endpoint newtab serves. The snapshot
// function is called on each render and must not block; it is nil when
// no monitor is configured.
func New(c *config.Config, snapshot func() status.Snapshot, stats func() proxmox.Stats, sky func() weather.Now, quotes func() rates.Table) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		snap := status.Snapshot{}
		if snapshot != nil {
			snap = snapshot()
		}
		pve := proxmox.Stats{}
		if stats != nil {
			pve = stats()
		}
		wx := weather.Now{}
		if sky != nil {
			wx = sky()
		}
		table := rates.Table{}
		if quotes != nil {
			table = quotes()
		}
		body, err := render(c, snap, pve, wx, table)
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
	// A browser that ignores the <link> asks for this by name; one that
	// reads it gets the SVG, which is sharp at any tab size.
	mux.HandleFunc("GET /favicon.svg", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "public, max-age=604800")
		w.Write([]byte(FaviconSVG))
	})
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=604800")
		w.Write(AppIcon(192))
	})
	mux.HandleFunc("GET /icon/{slug}", iconHandler(c.IconDir))
	// A phone that is told to add this to the home screen needs a name,
	// a colour and an icon, or it invents all three from a screenshot.
	if c.Theme.Image != "" {
		path := c.Theme.Image
		serve := func(w http.ResponseWriter, r *http.Request) {
			// One file named in the config, not a directory: there is no
			// path to traverse.
			//
			// The address carries the file's digest, so the answer can
			// never go stale and the browser is told to keep it for a
			// year. Fetching a background again on every new tab is what
			// made it appear a moment after the page.
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			http.ServeFile(w, r, path)
		}
		mux.HandleFunc("GET /background/{sum}", serve)
		// The plain name stays for a page rendered before the digest was
		// known, and for anyone who bookmarked it.
		mux.HandleFunc("GET /background", serve)
	}
	mux.HandleFunc("GET /manifest.webmanifest", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/manifest+json")
		w.Write(manifest(c))
	})
	for _, size := range []int{192, 512} {
		body := AppIcon(size)
		mux.HandleFunc(fmt.Sprintf("GET /app-%d.png", size), func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Cache-Control", "public, max-age=604800")
			w.Write(body)
		})
	}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("ok\n"))
	})
	return mux
}

// manifest is the web app manifest, built from the config so the name on
// a home screen is the name in the config.
func manifest(c *config.Config) []byte {
	// The colour a browser paints before the page has drawn anything:
	// the theme's own background, or the picture's mean colour when
	// there is a picture, so the wait is the same colour as the page.
	colour := c.Theme.Background
	if colour == "" {
		colour = "#141312"
	}
	if c.Average != "" {
		colour = darken(c.Average, c.Theme.ImageDim)
	}
	doc := map[string]any{
		"name":             c.Title,
		"short_name":       c.Title,
		"start_url":        "/",
		"display":          "standalone",
		"background_color": colour,
		"theme_color":      colour,
		"icons": []map[string]any{
			{"src": "/app-192.png", "sizes": "192x192", "type": "image/png"},
			{"src": "/app-512.png", "sizes": "512x512", "type": "image/png", "purpose": "any maskable"},
		},
	}
	body, _ := json.Marshal(doc)
	return body
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

// Demo renders the page against state a caller invented. It exists for
// `newtab demo`, which is how the screenshots are taken and how someone
// can see what the monitor adds before wiring one up.
// With inline set the icons are embedded, so the file stands on its own
// wherever it is opened.
func Demo(c *config.Config, snap status.Snapshot, pve proxmox.Stats, wx weather.Now, quotes rates.Table, inline bool) ([]byte, error) {
	return buildWith(c, inline, snap, pve, wx, quotes, "")
}

// Extension renders the page for a browser extension: icons embedded and
// the script in a file of its own, because an extension page refuses an
// inline one.
func Extension(c *config.Config, scriptName string) (page, script []byte, err error) {
	// Icons as files in an icons/ directory rather than a megabyte of
	// base64: the page a person unzips should be one they can open and
	// read, and a link they want to change should be findable in it.
	iconBase = "icons/"
	defer func() { iconBase = "" }()
	page, err = buildExtension(c, scriptName)
	return page, Script(), err
}

// Render returns the page as a file. With inline set, the icons are
// embedded in it and the result needs nothing else — no server, no
// network, no icon directory.
func Render(c *config.Config, inline bool) ([]byte, error) {
	if inline {
		return buildInline(c)
	}
	// A file has no live monitor behind it, so it shows no state.
	return render(c, status.Snapshot{}, proxmox.Stats{}, weather.Now{}, rates.Table{})
}
