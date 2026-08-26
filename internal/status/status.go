// Package status reads the uptime monitor next door and keeps its answer
// in memory.
//
// The page never asks the monitor anything while a browser is waiting:
// the poll runs on its own clock, the page reads the last answer, and a
// monitor that is down or slow costs the start page nothing. Nothing is
// written back — the monitor is the source of truth for what is up, and
// this is a reader.
package status

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Contract is the version of the monitor's document this package
// understands. The field is in the body, so a bump is visible here
// rather than in a 404.
const Contract = 1

// Check is the part of a monitor check the page can show.
type Check struct {
	Name string
	Up   bool
	// Muted checks are silenced deliberately; the page says nothing about
	// them rather than claiming they are fine.
	Muted     bool
	LatencyMS int
	// Uptime24h and Uptime7d are ratios in 0..1, as the monitor reports
	// them. Zero means the monitor said nothing, which is not the same
	// as an hour of downtime.
	Uptime24h float64
	Uptime7d  float64
	// Down is how long the outage has lasted. Zero unless Up is false.
	Down time.Duration
}

// Snapshot is what the poller last saw, indexed by check name and by the
// host of the address the monitor probes.
type Snapshot struct {
	byName map[string]Check
	byHost map[string]Check
}

// Lookup finds the check for a link: an explicit name from the config
// wins, otherwise the host of the link's own URL. The second is what
// makes a config with no check names work at all.
func (s Snapshot) Lookup(name, linkURL string) (Check, bool) {
	if s.byName == nil {
		return Check{}, false
	}
	if name != "" {
		c, ok := s.byName[name]
		return c, ok
	}
	u, err := url.Parse(linkURL)
	if err != nil {
		return Check{}, false
	}
	c, ok := s.byHost[strings.ToLower(u.Hostname())]
	return c, ok
}

// Fixed builds a snapshot from checks keyed by name. It is how a caller
// renders a page against known state — a test, or a preview — without a
// monitor to poll.
func Fixed(byName map[string]Check) Snapshot {
	snap := Snapshot{byName: map[string]Check{}, byHost: map[string]Check{}}
	for name, c := range byName {
		snap.byName[name] = c
	}
	return snap
}

// Poller keeps one Snapshot fresh.
type Poller struct {
	URL   string
	Every time.Duration

	mu   sync.RWMutex
	snap Snapshot
}

// Snapshot returns the last answer. It is never nil and never blocks on
// the network; before the first poll it is simply empty, and every row
// falls back to having nothing to say.
func (p *Poller) Snapshot() Snapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.snap
}

// Run polls until the context is cancelled. It returns no error: a
// monitor that cannot be reached is a page without dots, not a page that
// fails to start.
func (p *Poller) Run(ctx context.Context) {
	every := p.Every
	if every <= 0 {
		every = 30 * time.Second
	}
	client := &http.Client{Timeout: 5 * time.Second}
	for {
		if snap, err := p.fetch(ctx, client); err == nil {
			p.mu.Lock()
			p.snap = snap
			p.mu.Unlock()
		}
		// A failed poll keeps the previous answer rather than blanking
		// the page: a monitor restart is not an outage of everything.
		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
	}
}

type document struct {
	Version int `json:"version"`
	Checks  []struct {
		Name       string `json:"name"`
		URL        string `json:"url"`
		RemoteAddr string `json:"remote_addr"`
		Status     string `json:"status"`
		Muted      bool   `json:"muted"`
		LastProbe  *struct {
			DurationMS int `json:"duration_ms"`
		} `json:"last_probe"`
		Incident *struct {
			DurationMS int64 `json:"duration_ms"`
		} `json:"incident"`
		Uptime24h *struct {
			Ratio float64 `json:"ratio"`
		} `json:"uptime_24h"`
		Uptime7d *struct {
			Ratio float64 `json:"ratio"`
		} `json:"uptime_7d"`
	} `json:"checks"`
}

func (p *Poller) fetch(ctx context.Context, client *http.Client) (Snapshot, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.URL, nil)
	if err != nil {
		return Snapshot{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return Snapshot{}, err
	}
	defer resp.Body.Close()
	var doc document
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return Snapshot{}, err
	}
	snap := Snapshot{byName: map[string]Check{}, byHost: map[string]Check{}}
	for _, c := range doc.Checks {
		check := Check{Name: c.Name, Up: c.Status == "up", Muted: c.Muted}
		if c.LastProbe != nil {
			check.LatencyMS = c.LastProbe.DurationMS
		}
		if c.Uptime24h != nil {
			check.Uptime24h = c.Uptime24h.Ratio
		}
		if c.Uptime7d != nil {
			check.Uptime7d = c.Uptime7d.Ratio
		}
		if c.Incident != nil {
			check.Down = time.Duration(c.Incident.DurationMS) * time.Millisecond
		}
		snap.byName[c.Name] = check
		// Several checks can share a host — a site and its health
		// endpoint. First one wins, which is the one listed first in the
		// monitor's own config.
		if h := hostOf(c.URL, c.RemoteAddr); h != "" {
			if _, taken := snap.byHost[h]; !taken {
				snap.byHost[h] = check
			}
		}
	}
	return snap, nil
}

func hostOf(raw, remote string) string {
	if u, err := url.Parse(raw); err == nil && u.Hostname() != "" {
		return strings.ToLower(u.Hostname())
	}
	if host, _, ok := strings.Cut(remote, ":"); ok {
		return strings.ToLower(host)
	}
	return ""
}
