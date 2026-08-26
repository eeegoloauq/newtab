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
)

//go:embed page.html
var pageHTML string

//go:embed page.css
var pageCSS string

//go:embed page.js
var pageJS string

var pageTmpl = template.Must(template.New("page").Parse(pageHTML))

type pageView struct {
	Title  string
	Engine string
	CSS    template.CSS
	JS     template.JS
	Cards  []sectionView
	Lists  []sectionView
}

type sectionView struct {
	Name  string
	Links []linkView
}

type linkView struct {
	Name string
	URL  string
	Host string
	// Key is what the filter matches against: the name, the host and any
	// aliases, lowercased and joined. Matching happens in the browser
	// against this one string so the filter never touches the network.
	Key string
}

// render builds the whole document. Card sections are drawn first as a
// block, then the list sections flow as columns, regardless of how the
// two are interleaved in the config — the config's order is preserved
// within each of the two groups.
func render(c *config.Config) ([]byte, error) {
	v := pageView{
		Title:  c.Title,
		Engine: c.Search.Engine,
		CSS:    template.CSS(pageCSS),
		JS:     template.JS(pageJS),
	}
	for _, s := range c.Sections {
		sv := sectionView{Name: s.Name}
		for _, l := range s.Links {
			sv.Links = append(sv.Links, linkView{
				Name: l.Name,
				URL:  l.URL,
				Host: hostOf(l.URL),
				Key:  searchKey(l),
			})
		}
		if s.Style == config.StyleCards {
			v.Cards = append(v.Cards, sv)
		} else {
			v.Lists = append(v.Lists, sv)
		}
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

func searchKey(l config.Link) string {
	parts := append([]string{l.Name, hostOf(l.URL)}, l.Alias...)
	return strings.ToLower(strings.Join(parts, " "))
}
