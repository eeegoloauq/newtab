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
	"strings"

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
	// Text holds the few strings the page shows that are not a link name.
	// There is no translation framework and there should not be one for
	// four strings: the operator writes them in their own language, and
	// the defaults are English.
	Text     Text      `yaml:"text"`
	Search   Search    `yaml:"search"`
	Sections []Section `yaml:"sections"`
}

// Text is every user-visible string that does not come from the links.
// Keeping them here rather than in the template is what makes the page
// translatable at all — the binary ships no natural language.
type Text struct {
	// Search labels the input and is its placeholder.
	Search string `yaml:"search"`
	// Opens and WebSearch are announced to a screen reader as the filter
	// narrows: what Enter would open, or that it would go to the engine.
	// A sighted reader sees the same thing as an underline.
	Opens     string `yaml:"opens"`
	WebSearch string `yaml:"web_search"`
}

type Search struct {
	// Engine is a URL with one %s where the query goes. It is a template
	// rather than a provider name so switching engines never needs a code
	// change.
	Engine string `yaml:"engine"`
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
	// Alias adds words the filter should match besides the name, for the
	// links whose name is not what you type ("mail" for Gmail), including
	// the operator's own language.
	Alias []string `yaml:"alias"`
}

const (
	defaultColumns   = 4
	defaultLang      = "en"
	defaultSearch    = "Search"
	defaultOpens     = "Opens"
	defaultWebSearch = "Search the web"
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
	for i := range c.Sections {
		if c.Sections[i].Style == "" {
			c.Sections[i].Style = StyleList
		}
	}
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
