// Command newtab serves one page: the start page of the browser.
package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/eeegoloauq/newtab/internal/config"
	"github.com/eeegoloauq/newtab/internal/web"
)

// version is stamped at build time; a bare `go run` reports itself.
var version = "dev"

const usage = `newtab — start page

  newtab run <config.yaml>              serve the page
  newtab validate <config.yaml>         check the config, print what it holds
  newtab render <config.yaml> <out.html> write the page to a file
  newtab version
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "newtab:", err)
		os.Exit(1)
	}
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
