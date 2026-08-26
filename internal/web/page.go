// Package web renders the page. There is one page, it is rendered from
// the config on every request, and it carries its own CSS and JS inline —
// no build step, no bundle, no request to anywhere but this server.
package web

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/eeegoloauq/newtab/internal/config"
	"github.com/eeegoloauq/newtab/internal/icons"
	"github.com/eeegoloauq/newtab/internal/proxmox"
	"github.com/eeegoloauq/newtab/internal/rates"
	"github.com/eeegoloauq/newtab/internal/status"
	"github.com/eeegoloauq/newtab/internal/weather"
)

//go:embed page.html
var pageHTML string

//go:embed page.css
var pageCSS string

//go:embed page.js
var pageJS string

var pageTmpl = template.Must(template.New("page").Parse(pageHTML))

type pageView struct {
	// Rates is the line of quotes, or empty.
	Rates string
	// Weather is the temperature and the glyph beside the field, or empty
	// when no place is configured or nothing has been read yet.
	Weather *weatherView
	// Theme is the CSS custom properties the config overrides, already
	// checked to be colours and sizes.
	Theme template.CSS
	// Standalone drops the links a served page has and a file cannot use:
	// a manifest and an icon fetched by path mean nothing inside an
	// extension or a saved file.
	Standalone bool
	// Prefixes is the prefix table as JSON, for the one line of script
	// that reads it.
	Prefixes string
	// ScriptSrc names an external script instead of inlining it. Browser
	// extension pages forbid inline scripts, and that is the only place
	// this is used.
	ScriptSrc string
	Title     string
	Lang      string
	Text      config.Text
	Engine    string
	CSS       template.CSS
	JS        template.JS
	Columns   []columnView
}

// columnView is one column of the page. Dealing the sections into
// columns here, rather than letting CSS do it, is what keeps a section
// from jumping to another column when the filter hides a row.
// weatherView is what the page draws for the weather: a number and the
// name of a glyph. No word, so no language.
type weatherView struct {
	Temperature string
	Sky         string
	Night       bool
}

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
	// Down marks a row whose check is failing. The row says so twice: in
	// this colour and in the tail, because colour alone is not something
	// every reader can see.
	Down bool
	// TailHint explains the tail on hover, and it sits on the row rather
	// than on the numbers: a tooltip you have to find the right three
	// characters to hover over is a tooltip nobody sees.
	TailHint string
	// Tail is what a live row carries after the name: how long it has
	// been down, or a number about the thing itself. The host used to go
	// here and was removed — a truncated photos.example.com… told the
	// reader nothing they did not already know from the name. The slot
	// stays because it is the one place on the page where a value goes,
	// and it is the same slot on every row so the columns stay straight.
	Tail string
	// Pin lifts this row to the top of its section.
	Pin bool
	// Key is what the filter matches against: the name, the host and any
	// aliases, lowercased and joined. Matching happens in the browser
	// against this one string so the filter never touches the network.
	Key string
}

// render builds the whole document. Every section is a list and they all
// flow through the same columns, in config order: the operator's order is
// the reading order.
func render(c *config.Config, snap status.Snapshot, pve proxmox.Stats, wx weather.Now, quotes rates.Table) ([]byte, error) {
	return buildWith(c, false, snap, pve, wx, quotes, "")
}

// Script is the page's JavaScript, for the one caller that has to ship it
// as a separate file.
func Script() []byte { return []byte(pageJS) }

// buildInline is the same page with every icon embedded as a data: URI,
// so the result is a single file that works from disk, inside a browser
// extension, or anywhere the server is not reachable.
func buildInline(c *config.Config) ([]byte, error) {
	return build(c, true, status.Snapshot{}, proxmox.Stats{})
}

// buildExtension is the single-file page with its script pulled out, for
// a browser extension: extension pages refuse inline scripts.
func buildExtension(c *config.Config, script string) ([]byte, error) {
	return buildWith(c, true, status.Snapshot{}, proxmox.Stats{}, weather.Now{}, rates.Table{}, script)
}

func build(c *config.Config, inline bool, snap status.Snapshot, pve proxmox.Stats) ([]byte, error) {
	return buildWith(c, inline, snap, pve, weather.Now{}, rates.Table{}, "")
}

func buildWith(c *config.Config, inline bool, snap status.Snapshot, pve proxmox.Stats, wx weather.Now, quotes rates.Table, script string) ([]byte, error) {
	v := pageView{
		Standalone: inline,
		ScriptSrc:  script,
		Title:      c.Title,
		Lang:       c.Lang,
		Text:       c.Text,
		Engine:     c.Search.Engine,
		CSS:        template.CSS(pageCSS),
		JS:         template.JS(pageJS),
		Theme:      template.CSS(themeCSS(c.Theme)),
		Prefixes:   prefixJSON(c.Search.Prefixes),
	}
	// A page built as a file has no server behind it to have polled
	// anything, so it carries no weather.
	if !inline {
		v.Rates = quotes.Line()
	}
	if wx.OK && !inline {
		v.Weather = &weatherView{
			Temperature: strconv.Itoa(wx.Temperature) + "\u00b0",
			Sky:         string(wx.Sky),
			Night:       wx.Night,
		}
	}
	store := icons.Store{Dir: c.IconDir}
	var flat []sectionView
	for _, s := range c.Sections {
		sv := sectionView{Name: s.Name}
		for _, l := range s.Links {
			lv := linkView{
				Pin:  l.Pin,
				Name: l.Name,
				URL:  l.URL,
				Host: hostOf(l.URL),
				Key:  searchKey(l),
			}
			if s.Style == config.StyleLive {
				lv.Tail, lv.Down = state(snap, l, c.Text, c.Status.Tail)
			}
			// The hypervisor's own numbers win the slot on its row: they
			// say more about it than its latency ever will.
			if l.Name == c.Proxmox.Attach && !lv.Down && pve.OK {
				lv.Tail, lv.TailHint = hypervisor(pve, c.Text)
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
		// Pinned links come first, and the rest keep the order they were
		// written in.
		sort.SliceStable(sv.Links, func(i, j int) bool { return sv.Links[i].Pin && !sv.Links[j].Pin })
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

// state turns a monitor check into the two things a row shows. A link
// with no check says nothing at all: an empty tail is honest, a "?" is
// not.
func state(snap status.Snapshot, l config.Link, text config.Text, policy string) (tail string, down bool) {
	check, ok := snap.Lookup(l.Check, l.URL)
	switch {
	case !ok || check.Muted:
		return "", false
	case !check.Up:
		return strings.TrimSpace(text.Down + " " + compact(check.Down)), true
	}
	switch policy {
	case config.TailNever:
		return "", false
	case config.TailLatency:
		if check.LatencyMS > 0 {
			return strconv.Itoa(check.LatencyMS) + " ms", false
		}
		// A probe that landed inside a millisecond reports zero, which is
		// not "no answer" — that row deserves a number like every other.
		return "<1 ms", false
	case config.TailUptime24h:
		return percent(check.Uptime24h) + " 24h", false
	case config.TailUptime7d:
		return percent(check.Uptime7d) + " 7d", false
	}
	// Problems, the default: a row that has been up all day says nothing. A number that reads 100% every day is furniture, and the
	// eye stops seeing the one day it does not.
	if check.Uptime24h > 0 && check.Uptime24h < 1 {
		return percent(check.Uptime24h) + " 24h", false
	}
	return "", false
}

// hypervisor writes the three numbers twice: terse for the column, and
// spelled out for the tooltip.
func hypervisor(s proxmox.Stats, text config.Text) (tail, hint string) {
	tail = fmt.Sprintf("%d · %d%% · %d%%", s.Running, s.CPU, s.Memory)
	hint = fmt.Sprintf("%d %s · %d%% %s · %d%% %s",
		s.Running, text.Guests, s.CPU, text.CPU, s.Memory, text.Memory)
	return tail, hint
}

// percent writes a ratio the way it would be read aloud: 100%, 99.8%.
func percent(ratio float64) string {
	v := ratio * 100
	if v >= 99.95 {
		return "100%"
	}
	return strconv.FormatFloat(v, 'f', 1, 64) + "%"
}

// compact writes a duration the way someone glancing at it would say it.
// A tail that reads "1h23m45.6s" is a machine talking to itself.
func compact(d time.Duration) string {
	switch {
	case d <= 0:
		return ""
	case d < time.Hour:
		return strconv.Itoa(int(d.Minutes())) + "m"
	case d < 48*time.Hour:
		return strconv.Itoa(int(d.Hours())) + "h"
	}
	return strconv.Itoa(int(d.Hours()/24)) + "d"
}

// prefixJSON is the prefix table the browser reads. Marshalling it here
// rather than writing it into the script keeps the script a constant.
func prefixJSON(prefixes map[string]string) string {
	if len(prefixes) == 0 {
		return ""
	}
	body, err := json.Marshal(prefixes)
	if err != nil {
		return ""
	}
	return string(body)
}

// themeCSS turns the config's overrides into custom properties. Every
// value here was checked by the config: colours are hex, sizes are in
// range, and nothing else reaches this string.
func themeCSS(t config.Theme) string {
	var b strings.Builder
	b.WriteString(":root{")
	for prop, value := range map[string]string{
		"--bg":   t.Background,
		"--ink":  t.Ink,
		"--head": t.Muted,
		"--down": t.Down,
	} {
		if value != "" {
			fmt.Fprintf(&b, "%s:%s;", prop, value)
		}
	}
	if t.FontSize != 0 {
		fmt.Fprintf(&b, "--size:%dpx;", t.FontSize)
	}
	if t.Image != "" {
		fmt.Fprintf(&b, "--shade:rgba(0,0,0,%.2f);", t.ImageDim)
	}
	b.WriteString("}")
	if t.Image != "" {
		// The photograph goes behind a shade, because the page is text
		// and text on a photograph is the oldest way to make it
		// unreadable.
		b.WriteString("body{background:var(--shade) url(/background) center/cover fixed;background-blend-mode:darken;}")
		// A photograph has bright patches wherever it likes, and the
		// quiet inks disappear into them: measured over the bright part
		// of a photograph, headings fell to 2.6:1 against 7.4:1 for the
		// links. A shadow holds the text up, and the quietest inks are
		// promoted one step — on a picture there is no such thing as a
		// safely quiet grey.
		b.WriteString("body{text-shadow:0 1px 2px rgba(0,0,0,.65);}")
		b.WriteString("h2,.tail,.rates,.wx,.glyph,#q::placeholder{color:var(--dim);}")
	}
	return b.String()
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
	// Longest first, into the shortest column. Taking them in config
	// order instead left one column twice the height of its neighbours,
	// which wastes the screen the page exists to fit into. Order inside a
	// column is restored below, so reading down still follows the config.
	order := make([]int, len(sections))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return len(sections[order[a]].Links) > len(sections[order[b]].Links)
	})
	placed := make([][]int, n)
	for _, idx := range order {
		at := 0
		for i := 1; i < n; i++ {
			if load[i] < load[at] {
				at = i
			}
		}
		placed[at] = append(placed[at], idx)
		load[at] += len(sections[idx].Links) + 2
	}
	for i, idxs := range placed {
		sort.Ints(idxs)
		for _, idx := range idxs {
			cols[i].Sections = append(cols[i].Sections, sections[idx])
		}
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
