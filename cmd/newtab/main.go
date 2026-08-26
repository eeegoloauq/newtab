// Command newtab serves one page: the start page of the browser.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/eeegoloauq/newtab/internal/config"
	"github.com/eeegoloauq/newtab/internal/icons"
	"github.com/eeegoloauq/newtab/internal/web"
)

// version is stamped at build time; a bare `go run` reports itself.
var version = "dev"

const usage = `newtab — start page

  newtab run <config.yaml>              serve the page
  newtab validate <config.yaml>         check the config, print what it holds
  newtab render <config.yaml> <out.html> write the page to a file
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
	for _, s := range c.Sections {
		for _, l := range s.Links {
			// Re-fetching what we already have is a needless request to
			// someone else's server on every run.
			if !force && store.Path(l.Name) != "" {
				continue
			}
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
	return nil
}

func run(args []string) error {
	if len(args) == 0 {
		fmt.Print(usage)
		return nil
	}
	switch args[0] {
	case "version":
		fmt.Println(version)
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
		if len(args) < 3 {
			return fmt.Errorf("render needs a config file and an output file")
		}
		c, err := config.Load(args[1])
		if err != nil {
			return err
		}
		body, err := web.Render(c)
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
		fmt.Printf("newtab %s on http://%s\n", version, c.Listen)
		return srv.ListenAndServe()
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
