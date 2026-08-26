// Command newtab serves one page: the start page of the browser.
package main

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/eeegoloauq/newtab/internal/config"
	"github.com/eeegoloauq/newtab/internal/icons"
	"github.com/eeegoloauq/newtab/internal/proxmox"
	"github.com/eeegoloauq/newtab/internal/rates"
	"github.com/eeegoloauq/newtab/internal/status"
	"github.com/eeegoloauq/newtab/internal/weather"
	"github.com/eeegoloauq/newtab/internal/web"
)

// pseudoVersion matches what Go invents for a module built outside a
// release: that is the commit again in a longer costume, not a version.
var pseudoVersion = regexp.MustCompile(`[-.]\d{14}-[0-9a-f]{12}(\+dirty)?$`)

// version reads what the build itself recorded, so a release binary needs
// no ldflags and a `go run` cannot claim to be a release.
func version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	rev, when := "", ""
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) > 7 {
				rev = s.Value[:7]
			}
		case "vcs.time":
			if t, err := time.Parse(time.RFC3339, s.Value); err == nil {
				when = t.UTC().Format("2 Jan 2006")
			}
		}
	}
	tag := info.Main.Version
	if tag == "(devel)" || pseudoVersion.MatchString(tag) {
		tag = ""
	}
	switch {
	case tag != "" && rev != "":
		return tag + " (" + rev + ")"
	case tag != "":
		return tag
	case rev != "" && when != "":
		return rev + " \u00b7 " + when
	case rev != "":
		return rev
	}
	return "unknown"
}

const usage = `newtab — start page

  newtab run <config.yaml>              serve the page
  newtab validate <config.yaml>         check the config, print what it holds
  newtab render [-inline] <config.yaml> <out.html>
                                        write the page to a file; -inline
                                        embeds the icons so the file stands alone
  newtab icons [-force] <config.yaml>   fetch each site's own icon into icon_dir
  newtab extension <config.yaml> <dir>  write a browser extension that makes
                                        this page the new tab
  newtab demo [-config c.yaml] [-write out.html]
                                        the page on made-up state, for a look
                                        or for previewing a config of your own
  newtab version
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "newtab:", err)
		os.Exit(1)
	}
}

// fetchIcons walks the config and stores what each site serves. A site
// that refuses is reported and skipped: the page falls back to a
// monogram, and one hostile site must not fail the whole run.
// perLink is how long one site may take to give up its icon, across all
// the candidates it declares. An icon slower than this is not worth the
// run.
const perLink = 25 * time.Second

func fetchIcons(c *config.Config, force bool) error { return fetchIconsQuiet(c, force, false) }

// fetchIconsQuiet is the same walk without the per-link chatter, for the
// background pass a running server makes.
func fetchIconsQuiet(c *config.Config, force, quiet bool) error {
	store := icons.Store{Dir: c.IconDir}
	var missing []string
	tried := 0
	for _, s := range c.Sections {
		for _, l := range s.Links {
			// Re-fetching what we already have is a needless request to
			// someone else's server on every run. A stored file that is
			// not an image does not count as having one.
			if !force && store.Valid(l.Name) {
				continue
			}
			tried++
			// Each link gets its own budget. One deadline for the whole
			// run meant a config long enough to matter never finished:
			// a handful of sites that hang ate the entire allowance and
			// everything after them was never asked.
			ctx, cancel := context.WithTimeout(context.Background(), perLink)
			_, err := store.Fetch(ctx, l.Name, l.URL)
			cancel()
			if err != nil {
				missing = append(missing, l.Name)
				if !quiet {
					fmt.Printf("  -- %-22s %v\n", l.Name, err)
				}
				continue
			}
			if !quiet {
				fmt.Printf("  ok %-22s\n", l.Name)
			}
		}
	}
	if quiet {
		if len(missing) > 0 {
			fmt.Printf("icons: %d without one: %s\n", len(missing), strings.Join(missing, ", "))
		}
	} else {
		fmt.Printf("%d without an icon", len(missing))
		if len(missing) > 0 {
			fmt.Printf(": %s", strings.Join(missing, ", "))
		}
		fmt.Println()
	}
	// Silence plus exit 0 from a cron job means "all good". Every single
	// fetch failing is the opposite, and usually means no egress.
	if tried > 0 && len(missing) == tried {
		return fmt.Errorf("no icon could be fetched at all")
	}
	return nil
}

// ctxFor hands out a context for one background poller and records its
// cancel, so run() can stop them all on the way out.
func ctxFor(cancels *[]context.CancelFunc) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	*cancels = append(*cancels, cancel)
	return ctx
}

//go:embed manifest.json
var manifestJSON []byte

// writeExtension produces the three files Chrome and Firefox both accept
// as an unpacked extension. The page is the same page, with its icons
// embedded and its script in a file: an extension page refuses an inline
// script, and nothing here may reach the network anyway.
func writeExtension(c *config.Config, dir string) error {
	page, script, err := web.Extension(c, "newtab.js")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	files := map[string][]byte{
		"manifest.json": manifestJSON,
		"newtab.html":   page,
		"newtab.js":     script,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("wrote %s — load it as an unpacked extension\n", dir)
	return nil
}

//go:embed all:demo.yaml
var demoConfig []byte

// demo serves or writes the page against invented state, so the shipped
// screenshot and a first look both show what a monitor and a hypervisor
// add — without either of them existing.
func demo(args []string) error {
	// -config previews a config of your own against the same invented
	// state, which is the only way to see a down row without waiting for
	// something to break.
	path := ""
	if len(args) >= 2 && args[0] == "-config" {
		path, args = args[1], args[2:]
	}
	if path == "" {
		dir, err := os.MkdirTemp("", "newtab-demo")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		path = filepath.Join(dir, "demo.yaml")
		if err := os.WriteFile(path, demoConfig, 0o600); err != nil {
			return err
		}
	}
	c, err := config.Load(path)
	if err != nil {
		return err
	}
	if iconDir := os.Getenv("NEWTAB_DEMO_ICONS"); iconDir != "" {
		c.IconDir = iconDir
	}
	snap := status.Fixed(map[string]status.Check{
		"Files":  {Name: "Files", Up: true, LatencyMS: 4, Uptime24h: 1},
		"Photos": {Name: "Photos", Up: true, LatencyMS: 21, Uptime24h: 0.9982},
		"Music":  {Name: "Music", Up: true, LatencyMS: 8, Uptime24h: 1},
		"Git":    {Name: "Git", Down: 20 * time.Minute, Uptime24h: 0.986},
	})
	pve := proxmox.Stats{Running: 16, CPU: 29, Memory: 51, OK: true}
	wx := weather.Now{Temperature: 12, Sky: weather.Rain, OK: true}
	quotes := rates.Table{Base: "USD", Quotes: []rates.Quote{
		{Symbol: "EUR", Price: 0.857}, {Symbol: "BTC", Price: 78039},
	}}
	if len(args) == 2 && args[0] == "-write" {
		// A file is opened from disk, where /icon/... leads nowhere, so
		// the icons go inside it.
		body, err := web.Demo(c, snap, pve, wx, quotes, true)
		if err != nil {
			return err
		}
		return os.WriteFile(args[1], body, 0o644)
	}
	// Served, this is the ordinary server with invented readings behind
	// it: same icons, same background, same everything but the numbers.
	srv := &http.Server{
		Addr: c.Listen,
		Handler: web.New(c,
			func() status.Snapshot { return snap },
			func() proxmox.Stats { return pve },
			func() weather.Now { return wx },
			func() rates.Table { return quotes }),
		ReadHeaderTimeout: 5 * time.Second,
	}
	fmt.Printf("newtab demo on http://%s\n", c.Listen)
	return srv.ListenAndServe()
}

func run(args []string) error {
	if len(args) == 0 {
		fmt.Print(usage)
		return nil
	}
	switch args[0] {
	case "version":
		fmt.Println(version())
		return nil
	case "validate":
		if len(args) < 2 {
			return fmt.Errorf("validate needs a config file")
		}
		c, err := config.Load(args[1])
		if err != nil {
			return err
		}
		links := 0
		for _, s := range c.Sections {
			links += len(s.Links)
		}
		fmt.Printf("ok: %d sections, %d links, listening on %s\n", len(c.Sections), links, c.Listen)
		return nil
	case "icons":
		force := len(args) > 1 && args[1] == "-force"
		if force {
			args = append(args[:1], args[2:]...)
		}
		if len(args) < 2 {
			return fmt.Errorf("icons needs a config file")
		}
		c, err := config.Load(args[1])
		if err != nil {
			return err
		}
		if c.IconDir == "" {
			return fmt.Errorf("icon_dir is not set in the config")
		}
		return fetchIcons(c, force)
	case "extension":
		if len(args) < 3 {
			return fmt.Errorf("extension needs a config file and a directory")
		}
		c, err := config.Load(args[1])
		if err != nil {
			return err
		}
		return writeExtension(c, args[2])
	case "demo":
		return demo(args[1:])
	case "render":
		inline := len(args) > 1 && args[1] == "-inline"
		if inline {
			args = append(args[:1], args[2:]...)
		}
		if len(args) < 3 {
			return fmt.Errorf("render needs a config file and an output file")
		}
		c, err := config.Load(args[1])
		if err != nil {
			return err
		}
		body, err := web.Render(c, inline)
		if err != nil {
			return err
		}
		return os.WriteFile(args[2], body, 0o644)
	case "run":
		if len(args) < 2 {
			return fmt.Errorf("run needs a config file")
		}
		c, err := config.Load(args[1])
		if err != nil {
			return err
		}
		// The monitor is polled on its own clock; a request never waits
		// on it, and a monitor that is down costs this page nothing.
		var cancels []context.CancelFunc
		defer func() {
			for _, cancel := range cancels {
				cancel()
			}
		}()
		var snapshot func() status.Snapshot
		if c.Status.URL != "" {
			p := &status.Poller{URL: c.Status.URL, Every: c.Status.PollEvery()}
			go p.Run(ctxFor(&cancels))
			snapshot = p.Snapshot
		}
		var quotes func() rates.Table
		if c.Rates.Enabled() {
			p := &rates.Poller{
				Base:   c.Rates.Base,
				Fiat:   c.Rates.Fiat,
				Crypto: c.Rates.Crypto,
				Every:  c.Rates.PollEvery(),
			}
			go p.Run(ctxFor(&cancels))
			quotes = p.Table
		}
		var sky func() weather.Now
		if c.Weather.Enabled() {
			p := &weather.Poller{
				Latitude:   c.Weather.Latitude,
				Longitude:  c.Weather.Longitude,
				Fahrenheit: c.Weather.Fahrenheit,
				Every:      c.Weather.PollEvery(),
			}
			go p.Run(ctxFor(&cancels))
			sky = p.Now
		}
		var stats func() proxmox.Stats
		if c.Proxmox.URL != "" {
			p := &proxmox.Poller{
				URL:      c.Proxmox.URL,
				Token:    os.Getenv(c.Proxmox.TokenEnv),
				Every:    c.Proxmox.PollEvery(),
				Insecure: c.Proxmox.Insecure,
			}
			go p.Run(ctxFor(&cancels))
			stats = p.Stats
		}
		srv := &http.Server{
			Addr:              c.Listen,
			Handler:           web.New(c, snapshot, stats, sky, quotes),
			ReadHeaderTimeout: 5 * time.Second,
		}
		// Icons for links added since the last run are fetched in the
		// background, so adding a link is editing the config and nothing
		// else. Rendering still reads only the disk: a page must never
		// wait on someone else's server.
		if c.IconDir != "" {
			go func() {
				if err := fetchIconsQuiet(c, false, true); err != nil {
					fmt.Fprintln(os.Stderr, "icons:", err)
				}
			}()
		}
		fmt.Printf("newtab %s on http://%s\n", version(), c.Listen)
		return srv.ListenAndServe()
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
