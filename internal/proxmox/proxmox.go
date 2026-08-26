// Package proxmox reads a hypervisor's own numbers so one row of the
// page can carry them.
//
// Like the monitor client next door, it polls on its own clock and the
// page reads the last answer: a hypervisor that is slow to answer must
// not be something a browser waits for. It asks for one read-only
// endpoint and nothing else.
package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Stats is what one row can say about a hypervisor.
type Stats struct {
	// Running is how many containers and virtual machines are up.
	Running int
	CPU     int // percent
	Memory  int // percent
	// OK is false until a poll has succeeded. The row says nothing at
	// all rather than showing zeroes that look like an idle machine.
	OK bool
}

// Poller keeps one Stats fresh.
type Poller struct {
	URL   string
	Token string
	Every time.Duration
	// Insecure skips certificate verification. A Proxmox box answers
	// with its own self-signed certificate out of the box, and these are
	// three read-only numbers.
	Insecure bool

	mu   sync.RWMutex
	last Stats
}

func (p *Poller) Stats() Stats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.last
}

// Run polls until the context is cancelled. A failure keeps the previous
// answer: a hypervisor rebooting its API is not news for a start page.
func (p *Poller) Run(ctx context.Context) {
	every := p.Every
	if every <= 0 {
		every = 30 * time.Second
	}
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: p.Insecure},
		},
	}
	for {
		if s, err := p.fetch(ctx, client); err == nil {
			p.mu.Lock()
			p.last = s
			p.mu.Unlock()
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
	}
}

type resources struct {
	Data []struct {
		Type   string  `json:"type"`
		Status string  `json:"status"`
		CPU    float64 `json:"cpu"`
		Mem    int64   `json:"mem"`
		MaxMem int64   `json:"maxmem"`
	} `json:"data"`
}

func (p *Poller) fetch(ctx context.Context, client *http.Client) (Stats, error) {
	url := strings.TrimSuffix(p.URL, "/") + "/api2/json/cluster/resources"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return Stats{}, err
	}
	// A token, never a password: the account behind it is expected to
	// hold PVEAuditor and nothing more.
	req.Header.Set("Authorization", "PVEAPIToken="+p.Token)
	resp, err := client.Do(req)
	if err != nil {
		return Stats{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Stats{}, fmt.Errorf("%s: %s", url, resp.Status)
	}
	var doc resources
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return Stats{}, err
	}
	var s Stats
	for _, r := range doc.Data {
		switch r.Type {
		case "node":
			s.CPU = int(r.CPU*100 + 0.5)
			if r.MaxMem > 0 {
				s.Memory = int(float64(r.Mem)/float64(r.MaxMem)*100 + 0.5)
			}
		case "lxc", "qemu":
			// Guests that are stopped are not what the number is about:
			// it answers "how much is running right now".
			if r.Status == "running" {
				s.Running++
			}
		}
	}
	s.OK = true
	return s, nil
}

// Tail is the three numbers as one string, or "" when nothing has been
// read yet.
func (s Stats) Tail() string {
	if !s.OK {
		return ""
	}
	// Terse on purpose: the slot is one column wide, and "16 up · 29%
	// cpu · 51% ram" was cut off mid-word. On the hypervisor's own row,
	// guests then cpu then memory needs no labels.
	return fmt.Sprintf("%d · %d%% · %d%%", s.Running, s.CPU, s.Memory)
}
