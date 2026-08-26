// Package config turns the YAML file into the sections the page renders,
// and produces every error message the operator will ever see from it.
//
// The page has no admin UI on purpose: this file is the only way to add a
// link, so a bad file must fail loudly at `newtab validate`, not silently
// render an empty page at the moment the browser opens.
package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
)

// Style says how a section is drawn. Both are lists in the same columns —
// tiles were tried and thrown away, they turned a page of links into a
// launcher. A live section adds two things to each row: a status dot and
// a tail for the host or a number. A section moves between the two by
// editing one word.
const (
	StyleLive = "live"
	StyleList = "list"
)

type Config struct {
	Title  string `yaml:"title"`
	Listen string `yaml:"listen"`
	// IconDir holds icons fetched by `newtab icons`. Empty means every row
	// draws the browser's globe and no image is ever requested.
	IconDir string `yaml:"icon_dir"`
	// Columns is how many columns the sections are dealt into. The page
	// splits them on the server rather than leaving it to CSS: CSS
	// columns rebalance whenever content changes, so filtering threw
	// sections sideways on every keystroke.
	Columns int `yaml:"columns"`
	// Lang is the document language. It is what a screen reader switches
	// its pronunciation on, so it belongs to the config that names the
	// links, not to the code.
	Lang string `yaml:"lang"`
	// Proxmox attaches a hypervisor's own numbers to one row.
	Proxmox Proxmox `yaml:"proxmox"`
	// Status points at a lookout instance whose /api/status this page
	// reads. Empty means the page shows no state at all: this is the one
	// integration, and it is optional.
	Status Status `yaml:"status"`
	// Rates puts a line of exchange rates and crypto prices beside the
	// field. Both sources are open without an account.
	Rates Rates `yaml:"rates"`
	// Weather puts the current conditions of one place beside the field.
	// It reads Open-Meteo, which needs no account and no key.
	Weather Weather `yaml:"weather"`
	// Theme is the handful of things worth letting someone change without
	// touching the CSS. Anything more and the page stops being one page.
	Theme Theme `yaml:"theme"`
	// Text holds the few strings the page shows that are not a link name.
	// There is no translation framework and there should not be one for
	// four strings: the operator writes them in their own language, and
	// the defaults are English.
	Text     Text      `yaml:"text"`
	Search   Search    `yaml:"search"`
	Sections []Section `yaml:"sections"`
}

// Proxmox is a hypervisor to read three numbers from.
type Proxmox struct {
	URL string `yaml:"url"`
	// TokenEnv names the environment variable holding
	// user@realm!tokenid=secret. The secret is not in this file: the file
	// is in version control and the secret is not.
	TokenEnv string `yaml:"token_env"`
	// Attach is the link name whose row carries the numbers. They belong
	// on the row of the thing they describe, not in a panel of their own.
	Attach string `yaml:"attach"`
	Every  string `yaml:"every"`
	// Insecure accepts the self-signed certificate a Proxmox box ships
	// with. These are three read-only numbers on a LAN.
	Insecure bool `yaml:"insecure"`
}

// PollEvery is Every parsed, or 30s.
func (p Proxmox) PollEvery() time.Duration {
	if d, err := time.ParseDuration(p.Every); err == nil && d > 0 {
		return d
	}
	return 30 * time.Second
}

// Rates is what to quote and in what. Empty lists mean nothing is
// shown, which is the default.
type Rates struct {
	// Base is the currency everything is priced in; USD if unset.
	Base string `yaml:"base"`
	// Fiat are currency codes: one Base costs this much of each.
	Fiat []string `yaml:"fiat"`
	// Crypto are asset codes priced in Base.
	Crypto []string `yaml:"crypto"`
	Every  string   `yaml:"every"`
}

// Enabled reports whether anything was asked for.
func (r Rates) Enabled() bool { return len(r.Fiat) > 0 || len(r.Crypto) > 0 }

// PollEvery is Every parsed, or 30m. Currency rates are published once a
// day; crypto moves faster than anyone should watch from a start page.
func (r Rates) PollEvery() time.Duration {
	if d, err := time.ParseDuration(r.Every); err == nil && d > 0 {
		return d
	}
	return 30 * time.Minute
}

// Weather is one place to report. Empty latitude and longitude mean the
// page shows nothing, which is the default.
type Weather struct {
	Latitude   float64 `yaml:"latitude"`
	Longitude  float64 `yaml:"longitude"`
	Fahrenheit bool    `yaml:"fahrenheit"`
	Every      string  `yaml:"every"`
}

// Enabled reports whether a place was actually given. Zero, zero is a
// point in the Atlantic; nobody means it.
func (w Weather) Enabled() bool { return w.Latitude != 0 || w.Longitude != 0 }

// PollEvery is Every parsed, or 15m: weather that is a quarter of an
// hour old is still weather.
func (w Weather) PollEvery() time.Duration {
	if d, err := time.ParseDuration(w.Every); err == nil && d > 0 {
		return d
	}
	return 15 * time.Minute
}

// Theme overrides the page's colours and size. Every field is optional
// and an empty one keeps the built-in value.
type Theme struct {
	Background string `yaml:"background"`
	Ink        string `yaml:"ink"`
	Muted      string `yaml:"muted"`
	Down       string `yaml:"down"`
	// FontSize is the size of a link in pixels; everything else is
	// relative to it.
	FontSize int `yaml:"font_size"`
	// Image is a file on disk to put behind the page. The page was built
	// without one on purpose — text on a photograph is harder to read —
	// so it comes with a dimming layer that defaults to most of the way.
	Image    string  `yaml:"image"`
	ImageDim float64 `yaml:"image_dim"`
}

// Text is every user-visible string that does not come from the links.
// Keeping them here rather than in the template is what makes the page
// translatable at all — the binary ships no natural language.
type Text struct {
	// Search labels the input and is its placeholder.
	Search string `yaml:"search"`
	// Down is the word a row shows while its check is failing, before
	// how long it has been failing.
	Down string `yaml:"down"`
	// Guests, CPU and Memory name the hypervisor's three numbers. They
	// appear only in the tooltip: the row itself has no space for words,
	// and three bare numbers need saying once.
	Guests string `yaml:"guests"`
	CPU    string `yaml:"cpu"`
	Memory string `yaml:"memory"`
	// Opens and WebSearch are announced to a screen reader as the filter
	// narrows: what Enter would open, or that it would go to the engine.
	// A sighted reader sees the same thing as an underline.
	Opens     string `yaml:"opens"`
	WebSearch string `yaml:"web_search"`
}

// Status is the monitor to read. Nothing is ever written back to it.
type Status struct {
	URL string `yaml:"url"`
	// Tail decides what a healthy row says. A number that is the same
	// every day is not information, so the default speaks only when
	// something is off.
	Tail string `yaml:"tail"`
	// Every is a duration like "30s". The page never waits on the
	// monitor, so this only decides how stale the numbers can be.
	Every string `yaml:"every"`
}

// What a healthy row may show.
// The names say what the row shows, because the operator reads them once
// a year. "exceptions" and "quiet" were the first names and neither said
// anything: one sounded like error handling, the other like muting.
const (
	// TailProblems: nothing while the last day was perfect, the uptime
	// figure once it was not.
	TailProblems  = "problems"
	TailLatency   = "latency"
	TailUptime24h = "uptime24h"
	TailUptime7d  = "uptime7d"
	TailNever     = "never"
)

// PollEvery is Every parsed, or 30s.
func (s Status) PollEvery() time.Duration {
	if d, err := time.ParseDuration(s.Every); err == nil && d > 0 {
		return d
	}
	return 30 * time.Second
}

type Search struct {
	// Engine is a URL with one %s where the query goes. It is a template
	// rather than a provider name so switching engines never needs a code
	// change.
	Engine string `yaml:"engine"`
	// Prefixes send a query somewhere else when it starts with the key
	// and a space: "w cheese" to Wikipedia. Same %s rule as Engine.
	Prefixes map[string]string `yaml:"prefixes"`
}

type Section struct {
	Name  string `yaml:"name"`
	Style string `yaml:"style"`
	Links []Link `yaml:"links"`
}

type Link struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
	// Check names a lookout check when the automatic match by host is
	// wrong or ambiguous. Empty means "match by host, or no dot at all".
	Check string `yaml:"check"`
	// Pin puts a link first in its section, ahead of the config order.
	// It is the static answer to "put what I use most on top": no
	// history, no counting, and the row never moves on its own.
	Pin bool `yaml:"pin"`
	// Alias adds words the filter should match besides the name, for the
	// links whose name is not what you type ("mail" for Gmail), including
	// the operator's own language.
	Alias []string `yaml:"alias"`
}

// A colour and nothing else: this string is interpolated into CSS.
var hexColour = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

const (
	defaultColumns   = 4
	defaultLang      = "en"
	defaultSearch    = "Search"
	defaultOpens     = "Opens"
	defaultWebSearch = "Search the web"
	defaultDown      = "down"
	defaultGuests    = "running"
	defaultCPU       = "cpu"
	defaultMemory    = "memory"
	defaultTitle     = "newtab"
	defaultListen    = "127.0.0.1:5669"
	defaultEngine    = "https://www.google.com/search?q=%s"
)

// Load reads and validates the file. A Config it returns is safe to render.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	// Strict: a typo'd key that is silently ignored is a link that quietly
	// stops working — the failure mode this program exists to avoid.
	if err := yaml.UnmarshalWithOptions(raw, &c, yaml.Strict()); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	c.applyDefaults()
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &c, nil
}

func (c *Config) applyDefaults() {
	if c.Title == "" {
		c.Title = defaultTitle
	}
	if c.Listen == "" {
		c.Listen = defaultListen
	}
	if c.Search.Engine == "" {
		c.Search.Engine = defaultEngine
	}
	if c.Columns == 0 {
		c.Columns = defaultColumns
	}
	if c.Lang == "" {
		c.Lang = defaultLang
	}
	// Every field needs a default: an empty string in the YAML would
	// otherwise reach the page as an empty label.
	if c.Text.Search == "" {
		c.Text.Search = defaultSearch
	}
	if c.Text.Opens == "" {
		c.Text.Opens = defaultOpens
	}
	if c.Text.WebSearch == "" {
		c.Text.WebSearch = defaultWebSearch
	}
	if c.Text.Down == "" {
		c.Text.Down = defaultDown
	}
	if c.Theme.ImageDim == 0 {
		c.Theme.ImageDim = 0.72
	}
	if c.Text.Guests == "" {
		c.Text.Guests = defaultGuests
	}
	if c.Text.CPU == "" {
		c.Text.CPU = defaultCPU
	}
	if c.Text.Memory == "" {
		c.Text.Memory = defaultMemory
	}
	if c.Status.Tail == "" {
		c.Status.Tail = TailProblems
	}
	for i := range c.Sections {
		if c.Sections[i].Style == "" {
			c.Sections[i].Style = StyleList
		}
	}
}

func (c *Config) hasLink(name string) bool {
	for _, s := range c.Sections {
		for _, l := range s.Links {
			if l.Name == name {
				return true
			}
		}
	}
	return false
}

func (c *Config) validate() error {
	if !strings.Contains(c.Search.Engine, "%s") {
		return fmt.Errorf("search.engine %q has no %%s for the query", c.Search.Engine)
	}
	// The engine goes straight into location.href. A javascript: engine
	// in the config would be script running on the page.
	if u, err := url.Parse(c.Search.Engine); err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("search.engine %q is not an http(s) URL", c.Search.Engine)
	}
	if c.Status.URL != "" {
		u, err := url.Parse(c.Status.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("status.url %q is not an http(s) URL", c.Status.URL)
		}
		switch c.Status.Tail {
		case TailProblems, TailLatency, TailUptime24h, TailUptime7d, TailNever:
		default:
			return fmt.Errorf("status.tail %q: expected one of %s, %s, %s, %s, %s",
				c.Status.Tail, TailProblems, TailLatency, TailUptime24h, TailUptime7d, TailNever)
		}
		if c.Status.Every != "" {
			if _, err := time.ParseDuration(c.Status.Every); err != nil {
				return fmt.Errorf("status.every %q is not a duration like 30s", c.Status.Every)
			}
		}
	}
	for name, value := range map[string]string{
		"theme.background": c.Theme.Background,
		"theme.ink":        c.Theme.Ink,
		"theme.muted":      c.Theme.Muted,
		"theme.down":       c.Theme.Down,
	} {
		// The value goes into a stylesheet. Anything but a hex colour
		// would be an opening for whatever else fits in a CSS value.
		if value != "" && !hexColour.MatchString(value) {
			return fmt.Errorf("%s %q is not a hex colour like #141312", name, value)
		}
	}
	if c.Theme.FontSize != 0 && (c.Theme.FontSize < 12 || c.Theme.FontSize > 28) {
		return fmt.Errorf("theme.font_size is %d: expected 12 to 28", c.Theme.FontSize)
	}
	if c.Theme.ImageDim < 0 || c.Theme.ImageDim > 1 {
		return fmt.Errorf("theme.image_dim is %v: expected 0 to 1", c.Theme.ImageDim)
	}
	if c.Theme.Image != "" {
		if _, err := os.Stat(c.Theme.Image); err != nil {
			return fmt.Errorf("theme.image %q: %w", c.Theme.Image, err)
		}
	}
	if c.Rates.Enabled() {
		for _, code := range append(append([]string{}, c.Rates.Fiat...), c.Rates.Crypto...) {
			if len(code) < 2 || len(code) > 8 || strings.ContainsAny(code, " /\\?&") {
				return fmt.Errorf("rates: %q is not a currency or asset code", code)
			}
		}
		if c.Rates.Every != "" {
			if _, err := time.ParseDuration(c.Rates.Every); err != nil {
				return fmt.Errorf("rates.every %q is not a duration like 30m", c.Rates.Every)
			}
		}
	}
	if c.Weather.Enabled() {
		if c.Weather.Latitude < -90 || c.Weather.Latitude > 90 {
			return fmt.Errorf("weather.latitude %v is not a latitude", c.Weather.Latitude)
		}
		if c.Weather.Longitude < -180 || c.Weather.Longitude > 180 {
			return fmt.Errorf("weather.longitude %v is not a longitude", c.Weather.Longitude)
		}
		if c.Weather.Every != "" {
			if _, err := time.ParseDuration(c.Weather.Every); err != nil {
				return fmt.Errorf("weather.every %q is not a duration like 15m", c.Weather.Every)
			}
		}
	}
	if c.Proxmox.URL != "" {
		u, err := url.Parse(c.Proxmox.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("proxmox.url %q is not an http(s) URL", c.Proxmox.URL)
		}
		if c.Proxmox.TokenEnv == "" {
			return fmt.Errorf("proxmox.token_env is not set: the token does not belong in this file")
		}
		if os.Getenv(c.Proxmox.TokenEnv) == "" {
			return fmt.Errorf("$%s is empty: no Proxmox token to read with", c.Proxmox.TokenEnv)
		}
		if c.Proxmox.Attach == "" {
			return fmt.Errorf("proxmox.attach is not set: name the link whose row shows the numbers")
		}
		if !c.hasLink(c.Proxmox.Attach) {
			return fmt.Errorf("proxmox.attach %q is not a link in any section", c.Proxmox.Attach)
		}
		if c.Proxmox.Every != "" {
			if _, err := time.ParseDuration(c.Proxmox.Every); err != nil {
				return fmt.Errorf("proxmox.every %q is not a duration like 30s", c.Proxmox.Every)
			}
		}
	}
	for key, engine := range c.Search.Prefixes {
		if key == "" || strings.ContainsAny(key, " \t") {
			return fmt.Errorf("search.prefixes: %q is not a word you can type before a space", key)
		}
		if !strings.Contains(engine, "%s") {
			return fmt.Errorf("search.prefixes[%s] %q has no %%s for the query", key, engine)
		}
		if u, err := url.Parse(engine); err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("search.prefixes[%s] %q is not an http(s) URL", key, engine)
		}
	}
	// A typo here is a service that will not start; catching it in
	// validate is the whole point of the command.
	if _, _, err := net.SplitHostPort(c.Listen); err != nil {
		return fmt.Errorf("listen %q is not host:port", c.Listen)
	}
	// An icon_dir that does not exist is a page that silently has no
	// icons, forever.
	if c.IconDir != "" {
		if info, err := os.Stat(c.IconDir); err != nil {
			return fmt.Errorf("icon_dir %q: %w", c.IconDir, err)
		} else if !info.IsDir() {
			return fmt.Errorf("icon_dir %q is not a directory", c.IconDir)
		}
	}
	if c.Columns < 1 || c.Columns > 8 {
		return fmt.Errorf("columns is %d: expected 1 to 8", c.Columns)
	}
	if len(c.Sections) == 0 {
		return fmt.Errorf("no sections: the page would be empty")
	}
	seenSection := map[string]bool{}
	seenLink := map[string]bool{}
	for _, s := range c.Sections {
		if s.Name == "" {
			return fmt.Errorf("a section has no name")
		}
		if seenSection[s.Name] {
			return fmt.Errorf("section %q appears twice", s.Name)
		}
		seenSection[s.Name] = true
		if s.Style != StyleLive && s.Style != StyleList {
			return fmt.Errorf("section %q: style %q is neither %q nor %q", s.Name, s.Style, StyleLive, StyleList)
		}
		if len(s.Links) == 0 {
			return fmt.Errorf("section %q has no links", s.Name)
		}
		for _, l := range s.Links {
			if l.Name == "" {
				return fmt.Errorf("section %q: a link has no name", s.Name)
			}
			// Two links with the same name make the filter ambiguous and
			// make a lookout check impossible to attribute.
			if seenLink[l.Name] {
				return fmt.Errorf("link %q appears twice", l.Name)
			}
			seenLink[l.Name] = true
			u, err := url.Parse(l.URL)
			if err != nil || u.Scheme == "" || u.Host == "" {
				return fmt.Errorf("link %q: %q is not an absolute URL", l.Name, l.URL)
			}
			if u.Scheme != "http" && u.Scheme != "https" {
				return fmt.Errorf("link %q: scheme %q is not http(s)", l.Name, u.Scheme)
			}
		}
	}
	return nil
}
