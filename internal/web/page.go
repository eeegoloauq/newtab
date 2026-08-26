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
	"unicode"

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
	Title    string
	Lang     string
	Text     config.Text
	Engine   string
	CSS      template.CSS
	JS       template.JS
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
	// Icon is the path to serve the site's own icon from, or "" when we
	// have none. The check happens here, at render time, so the browser
	// is never asked to load an image that does not exist.
	Icon string
	// Mono is the letter drawn in an icon's place. It keeps the left edge
	// of every list straight, which is the whole reason the icons are
	// there.
	Mono string
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
func render(c *config.Config) ([]byte, error) {
	v := pageView{
		Title:  c.Title,
		Lang:   c.Lang,
		Text:   c.Text,
		Engine: c.Search.Engine,
		CSS:    template.CSS(pageCSS),
		JS:     template.JS(pageJS),
	}
	store := icons.Store{Dir: c.IconDir}
	for _, s := range c.Sections {
		sv := sectionView{Name: s.Name}
		for _, l := range s.Links {
			lv := linkView{
				Name: l.Name,
				URL:  l.URL,
				Host: hostOf(l.URL),
				Key:  searchKey(l),
				Mono: mono(l.Name),
			}
			if store.Valid(l.Name) {
				lv.Icon = "/icon/" + icons.Slug(l.Name)
			}
			sv.Links = append(sv.Links, lv)
		}
		sv.Live = s.Style == config.StyleLive
		v.Sections = append(v.Sections, sv)
	}
	var buf bytes.Buffer
	if err := pageTmpl.Execute(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(u.Hostname(), "www.")
}

// mono is the first letter of the name, upper-cased. Cyrillic and Latin
// both work; anything else falls back to a dot rather than a box glyph.
func mono(name string) string {
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return string(unicode.ToUpper(r))
		}
	}
	return "\u00b7"
}

func searchKey(l config.Link) string {
	parts := append([]string{l.Name, hostOf(l.URL)}, l.Alias...)
	return strings.ToLower(strings.Join(parts, " "))
}
