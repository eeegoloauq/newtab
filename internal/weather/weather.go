// Package weather reads the current conditions for one place.
//
// It uses Open-Meteo, which needs no account and no key, and it polls on
// its own clock like every other reader here: the page shows the last
// answer and never waits for one.
package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// Sky is the condition reduced to the few kinds worth drawing. The page
// shows a glyph rather than a word, so it needs no language.
type Sky string

const (
	Clear  Sky = "clear"
	Cloudy Sky = "cloudy"
	Fog    Sky = "fog"
	Rain   Sky = "rain"
	Snow   Sky = "snow"
	Storm  Sky = "storm"
)

// Now is the current conditions. OK is false until a poll has landed.
type Now struct {
	Temperature int
	Sky         Sky
	Night       bool
	OK          bool
}

type Poller struct {
	Latitude, Longitude float64
	// Fahrenheit asks the service for the other unit rather than
	// converting here, so the rounding matches what it would report.
	Fahrenheit bool
	Every      time.Duration

	mu        sync.RWMutex
	last      Now
	updatedAt time.Time
}

func (p *Poller) Now() Now {
	p.mu.RLock()
	defer p.mu.RUnlock()
	maxAge := time.Hour
	if p.Every*2 > maxAge {
		maxAge = p.Every * 2
	}
	if p.last.OK && time.Since(p.updatedAt) > maxAge {
		return Now{}
	}
	return p.last
}

// Run polls until cancellation. Brief failures keep recent weather; Now hides
// readings older than an hour or two configured polling intervals.
func (p *Poller) Run(ctx context.Context) {
	every := p.Every
	if every <= 0 {
		every = 15 * time.Minute
	}
	client := &http.Client{Timeout: 10 * time.Second}
	for {
		if err := p.poll(ctx, client); err != nil && ctx.Err() == nil {
			log.Printf("weather: refresh failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
	}
}

func (p *Poller) poll(ctx context.Context, client *http.Client) error {
	n, err := p.fetch(ctx, client)
	if err != nil {
		return err
	}
	p.mu.Lock()
	p.last = n
	p.updatedAt = time.Now()
	p.mu.Unlock()
	return nil
}

const endpoint = "https://api.open-meteo.com/v1/forecast"

type document struct {
	Current struct {
		Temperature float64 `json:"temperature_2m"`
		Code        int     `json:"weather_code"`
		IsDay       int     `json:"is_day"`
	} `json:"current"`
}

func (p *Poller) fetch(ctx context.Context, client *http.Client) (Now, error) {
	// Two decimals is about a kilometre, which is as much as weather
	// knows and rather less than the service needs to know about where
	// its caller lives.
	q := url.Values{
		"latitude":  {strconv.FormatFloat(p.Latitude, 'f', 2, 64)},
		"longitude": {strconv.FormatFloat(p.Longitude, 'f', 2, 64)},
		"current":   {"temperature_2m,weather_code,is_day"},
		"timezone":  {"auto"},
	}
	if p.Fahrenheit {
		q.Set("temperature_unit", "fahrenheit")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint+"?"+q.Encode(), nil)
	if err != nil {
		return Now{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return Now{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Now{}, fmt.Errorf("%s: %s", endpoint, resp.Status)
	}
	var doc document
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return Now{}, err
	}
	return Now{
		Temperature: int(doc.Current.Temperature + 0.5),
		Sky:         skyOf(doc.Current.Code),
		Night:       doc.Current.IsDay == 0,
		OK:          true,
	}, nil
}

// skyOf folds a WMO weather code into what the page can draw. The exact
// distinctions the code makes — drizzle against light rain, and so on —
// are not visible at the size of a glyph.
func skyOf(code int) Sky {
	switch {
	case code == 0 || code == 1:
		return Clear
	case code == 2 || code == 3:
		return Cloudy
	case code == 45 || code == 48:
		return Fog
	case code >= 51 && code <= 67, code >= 80 && code <= 82:
		return Rain
	case code >= 71 && code <= 77, code == 85 || code == 86:
		return Snow
	case code >= 95:
		return Storm
	}
	return Cloudy
}
