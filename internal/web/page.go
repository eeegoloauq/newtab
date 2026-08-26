// Package web renders the page. There is one page, it is rendered from
// the config on every request, and it carries its own CSS and JS inline —
// no build step, no bundle, no request to anywhere but this server.
package web

import (
	"bytes"
	_ "embed"
	"html/template"
	"net/url"
	"strings"

	"github.com/eeegoloauq/newtab/internal/config"
	"github.com/eeegoloauq/newtab/internal/icons"
)

//go:embed page.html
var pageHTML string

//go:embed page.css
var pageCSS string

//go:embed page.js
var pageJS string

var pageTmpl = template.Must(template.New("page").Parse(pageHTML))

type pageView struct {
	Title   string
	Lang    string
	Text    config.Text
	Engine  string
	CSS     template.CSS
	JS      template.JS
	Columns []columnView
}

// columnView is one column of the page. Dealing the sections into
// columns here, rather than letting CSS do it, is what keeps a section
// from jumping to another column when the filter hides a row.
type columnView struct {
	Sections []sectionView
}

type sectionView struct {
	Name string
	// Live marks a section of things that can be down: its rows carry a
	// status dot and a tail. Everything else is a bookmark: icon, name.
	Live  bool
	Links []linkView
}

type linkView struct {
	Name string
	URL  string
	Host string
	// Icon is where the row's image comes from: a path on this server, or
	// the whole image as a data: URI in the single-file export. The check
	// happens here, at render time, so the browser is never asked to load
	// an image that does not exist.
	//
	// It is a template.URL because html/template refuses a data: URI in
	// src otherwise. The value is ours: either a path we built from a
	// slug, or a file we read from the icon directory.
	Icon template.URL
	// Tail is what a live row carries after the name: how long it has
	// been down, or a number about the thing itself. The host used to go
	// here and was removed — a truncated photo.cdn.egor-solo… told the
	// reader nothing they did not already know from the name. The slot
	// stays because it is the one place on the page where a value goes,
	// and it is the same slot on every row so the columns stay straight.
	Tail string
	// Key is what the filter matches against: the name, the host and any
	// aliases, lowercased and joined. Matching happens in the browser
	// against this one string so the filter never touches the network.
	Key string
}

// render builds the whole document. Every section is a list and they all
// flow through the same columns, in config order: the operator's order is
// the reading order.
func render(c *config.Config) ([]byte, error) { return build(c, false) }

// buildInline is the same page with every icon embedded as a data: URI,
// so the result is a single file that works from disk, inside a browser
// extension, or anywhere the server is not reachable.
func buildInline(c *config.Config) ([]byte, error) { return build(c, true) }

func build(c *config.Config, inline bool) ([]byte, error) {
	v := pageView{
		Title:  c.Title,
		Lang:   c.Lang,
		Text:   c.Text,
		Engine: c.Search.Engine,
		CSS:    template.CSS(pageCSS),
		JS:     template.JS(pageJS),
	}
	store := icons.Store{Dir: c.IconDir}
	var flat []sectionView
	for _, s := range c.Sections {
		sv := sectionView{Name: s.Name}
		for _, l := range s.Links {
			lv := linkView{
				Name: l.Name,
				URL:  l.URL,
				Host: hostOf(l.URL),
				Key:  searchKey(l),
			}
			if store.Valid(l.Name) {
				lv.Icon = template.URL("/icon/" + icons.Slug(l.Name))
				if inline {
					// An icon that cannot be read is not worth failing a
					// whole page over: the row falls back to the globe.
					if uri, err := icons.DataURI(store.Icon(l.Name)); err == nil {
						lv.Icon = template.URL(uri)
					} else {
						lv.Icon = ""
					}
				}
			}
			sv.Links = append(sv.Links, lv)
		}
		sv.Live = s.Style == config.StyleLive
		flat = append(flat, sv)
	}
	v.Columns = deal(flat, c.Columns)
	var buf bytes.Buffer
	if err := pageTmpl.Execute(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// deal spreads sections over n columns in config order, keeping the
// columns near equal in height. Height is counted in rows plus one for
// the heading, which is what the reader actually sees.
func deal(sections []sectionView, n int) []columnView {
	// A Config built in code rather than loaded from a file has no
	// defaults applied, and rendering must not depend on them.
	if n < 1 {
		n = 1
	}
	cols := make([]columnView, n)
	load := make([]int, n)
	for _, s := range sections {
		// Order matters more than perfect balance: a section may only go
		// into the first column that is not taller than the others, so
		// reading down and across still follows the config.
		at := 0
		for i := 1; i < n; i++ {
			if load[i] < load[at] {
				at = i
			}
		}
		cols[at].Sections = append(cols[at].Sections, s)
		load[at] += len(s.Links) + 2
	}
	return cols
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(u.Hostname(), "www.")
}

func searchKey(l config.Link) string {
	parts := append([]string{l.Name, hostOf(l.URL)}, l.Alias...)
	return strings.ToLower(strings.Join(parts, " "))
}
