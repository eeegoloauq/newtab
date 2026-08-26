// Command newtab serves one page: the start page of the browser.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/eeegoloauq/newtab/internal/config"
	"github.com/eeegoloauq/newtab/internal/icons"
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
func fetchIcons(c *config.Config, force bool) error {
	store := icons.Store{Dir: c.IconDir}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
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
			if _, err := store.Fetch(ctx, l.Name, l.URL); err != nil {
				missing = append(missing, l.Name)
				fmt.Printf("  -- %-22s %v\n", l.Name, err)
				continue
			}
			fmt.Printf("  ok %-22s\n", l.Name)
		}
	}
	fmt.Printf("%d without an icon", len(missing))
	if len(missing) > 0 {
		fmt.Printf(": %s", strings.Join(missing, ", "))
	}
	fmt.Println()
	// Silence plus exit 0 from a cron job means "all good". Every single
	// fetch failing is the opposite, and usually means no egress.
	if tried > 0 && len(missing) == tried {
		return fmt.Errorf("no icon could be fetched at all")
	}
	return nil
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
		srv := &http.Server{
			Addr:              c.Listen,
			Handler:           web.New(c),
			ReadHeaderTimeout: 5 * time.Second,
		}
		fmt.Printf("newtab %s on http://%s\n", version(), c.Listen)
		return srv.ListenAndServe()
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
