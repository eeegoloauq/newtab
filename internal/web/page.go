// Package web renders the page. There is one page, it is rendered from
// the config on every request, and it carries its own CSS and JS inline —
// no build step, no bundle, no request to anywhere but this server.
package web

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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
	// Favicon is a path when a server is behind the page and the icon
	// itself when there is not: a saved file and an extension page have
	// nowhere to fetch /favicon.svg from, and a tab with no icon looks
	// like a page that failed to load.
	Favicon template.URL
	// Colour is what the page paints its canvas with, announced in the
	// head so a browser — or an extension opening this page — can paint
	// the same thing while it waits.
	Colour string
	// Scheme is what the browser paints before the stylesheet is parsed.
	// Getting it wrong is a black flash on a light page and a white one
	// on a dark page, and it is decided by one word in the head.
	Scheme string
	// Background is the address of the picture behind the page, so the
	// head can ask for it before the stylesheet is parsed. Empty when
	// there is none, or when the page carries it inside itself.
	Background string
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

// iconBase is where a rendered page looks for icons. Empty means the
// server's own /icon/ route; "icons/" means files next to the page,
// which is what the extension ships so its HTML stays small enough to
// read and edit.
var iconBase string

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
		Theme:      template.CSS(themeCSS(c.Theme, c.Fingerprint, c.Average, inline)),
		Favicon:    faviconHref(inline),
		Background: backgroundHref(c, inline),
		Scheme:     scheme(c.Theme.Background),
		Colour:     canvasColour(c),
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
				if iconBase != "" {
					lv.Icon = template.URL(iconBase + filepath.Base(store.Icon(l.Name)))
				} else if inline {
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

// dataURI reads a file as a data: URI, for the standalone page.
func dataURI(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	kind := http.DetectContentType(body)
	if !strings.HasPrefix(kind, "image/") {
		return "", fmt.Errorf("%s: %s is not an image", path, kind)
	}
	return "data:" + kind + ";base64," + base64.StdEncoding.EncodeToString(body), nil
}

// backgroundPath is where the picture lives: its name carries the
// digest, so a changed picture is a changed URL and an unchanged one is
// never asked about twice.
func backgroundPath(fingerprint string) string {
	if fingerprint == "" {
		return "/background"
	}
	// A segment, not a suffix: the router's patterns take a wildcard
	// only as a whole segment.
	return "/background/" + fingerprint
}

func backgroundHref(c *config.Config, inline bool) string {
	if c.Theme.Image == "" || inline {
		return ""
	}
	return backgroundPath(c.Fingerprint)
}

// darken multiplies a hex colour towards black, the way the shade layer
// does to the picture.
func darken(hex string, dim float64) string {
	var r, g, b int
	if _, err := fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b); err != nil {
		return hex
	}
	k := 1 - dim
	return fmt.Sprintf("#%02x%02x%02x", int(float64(r)*k), int(float64(g)*k), int(float64(b)*k))
}

// canvasColour is the page's own background: the theme's colour, or the
// picture's mean colour darkened the way the picture is.
func canvasColour(c *config.Config) string {
	if c.Average != "" {
		return darken(c.Average, c.Theme.ImageDim)
	}
	if c.Theme.Background != "" {
		return c.Theme.Background
	}
	return "#141312"
}

// scheme reads the configured background and says whether the page is a
// dark one or a light one. Anything the config did not set is dark,
// which is what the built-in palette is.
func scheme(background string) string {
	if len(background) != 7 {
		return "dark"
	}
	var r, g, b int
	if _, err := fmt.Sscanf(background, "#%02x%02x%02x", &r, &g, &b); err != nil {
		return "dark"
	}
	// Rough brightness is enough for a yes-or-no about a background.
	if (r*299+g*587+b*114)/1000 > 128 {
		return "light"
	}
	return "dark"
}

func faviconHref(inline bool) template.URL {
	if !inline {
		return "/favicon.svg"
	}
	return template.URL("data:image/svg+xml," + url.PathEscape(FaviconSVG))
}

// themeCSS turns the config's overrides into custom properties. Every
// value here was checked by the config: colours are hex, sizes are in
// range, and nothing else reaches this string.
func themeCSS(t config.Theme, fingerprint, average string, inline bool) string {
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
		// Two layers: the picture, and underneath it the same picture at
		// the size of a stamp, embedded in this stylesheet. The stamp is
		// there for the moment before the real one has arrived.
		// The picture is behind a URL that names its contents, so the
		// browser may keep it for a year and a new tab paints it from
		// disk rather than fetching it again. A page that opens dozens
		// of times a day cannot afford a network round trip for its
		// background, and the round trip — not the file size — was what
		// showed as a flash.
		src := backgroundPath(fingerprint)
		if inline {
			// A file has no server to fetch it from, so the picture
			// travels inside the file or not at all.
			if data, err := dataURI(t.Image); err == nil {
				src = data
			}
		}
		// On the root element, not on the body: the body is a column in
		// the middle of the window, and a background on it stops where
		// the column stops — which is exactly how it looked, a band of
		// picture with flat colour either side.
		//
		// The shade is a flat gradient over the picture rather than a
		// blend mode: two layers on one element, painted in the order
		// they are written, nothing to reason about.
		shade := fmt.Sprintf("rgba(0,0,0,%.2f)", t.ImageDim)
		colour := "#000000"
		if average != "" {
			// The picture's own mean colour, darkened the same amount,
			// is what fills the canvas until the picture is decoded.
			colour = darken(average, t.ImageDim)
		}
		fmt.Fprintf(&b, "html{background-color:%s;"+
			"background-image:linear-gradient(%s,%s),url(%s);"+
			"background-size:cover;background-position:center;background-attachment:fixed;}"+
			"body{background:none;}", colour, shade, shade, src)
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
