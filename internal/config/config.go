// Package config turns the YAML file into the sections the page renders,
// and produces every error message the operator will ever see from it.
//
// The page has no admin UI on purpose: this file is the only way to add a
// link, so a bad file must fail loudly at `newtab validate`, not silently
// render an empty page at the moment the browser opens.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
)

// Style says how a section is drawn. Two styles, chosen deliberately:
// things with a live status earn a card, plain bookmarks are a list. A
// section moves between the two by editing one word.
const (
	StyleCards = "cards"
	StyleList  = "list"
)

type Config struct {
	Title    string    `yaml:"title"`
	Listen   string    `yaml:"listen"`
	Search   Search    `yaml:"search"`
	Sections []Section `yaml:"sections"`
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
	// links whose name is not what you type ("почта" for Gmail).
	Alias []string `yaml:"alias"`
}

const (
	defaultTitle  = "newtab"
	defaultListen = "127.0.0.1:5669"
	defaultEngine = "https://www.google.com/search?q=%s"
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
		if s.Style != StyleCards && s.Style != StyleList {
			return fmt.Errorf("section %q: style %q is neither %q nor %q", s.Name, s.Style, StyleCards, StyleList)
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
