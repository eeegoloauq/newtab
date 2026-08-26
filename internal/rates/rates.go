// Package rates reads exchange rates and crypto prices.
//
// Two sources, both open without an account: exchangerate-api's free
// endpoint for currencies, and Coinbase's public spot price for crypto.
// Like the other readers here it polls on its own clock and the page
// shows the last answer.
package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Quote is one line's worth: what it is, and what it costs in the base
// currency.
type Quote struct {
	Symbol string
	Price  float64
}

// Table is the last reading. Empty until a poll has landed, which is how
// the page knows to show nothing.
type Table struct {
	Base   string
	Quotes []Quote
}

type Poller struct {
	// Base is the currency everything is priced in.
	Base string
	// Fiat are currency codes; one unit of Base costs this much of each.
	Fiat []string
	// Crypto are asset codes priced in Base.
	Crypto []string
	Every  time.Duration

	mu   sync.RWMutex
	last Table
}

func (p *Poller) Table() Table {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.last
}

// Run polls until the context is cancelled. Rates that are an hour old
// are still rates, and a failed poll keeps the previous answer.
func (p *Poller) Run(ctx context.Context) {
	every := p.Every
	if every <= 0 {
		every = 30 * time.Minute
	}
	client := &http.Client{Timeout: 10 * time.Second}
	for {
		if t, err := p.fetch(ctx, client); err == nil {
			p.mu.Lock()
			p.last = t
			p.mu.Unlock()
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
	}
}

const (
	fiatEndpoint   = "https://open.er-api.com/v6/latest/"
	cryptoEndpoint = "https://api.coinbase.com/v2/prices/"
)

func (p *Poller) fetch(ctx context.Context, client *http.Client) (Table, error) {
	base := strings.ToUpper(p.Base)
	if base == "" {
		base = "USD"
	}
	t := Table{Base: base}

	if len(p.Fiat) > 0 {
		rates, err := p.fiat(ctx, client, base)
		if err != nil {
			return Table{}, err
		}
		for _, code := range p.Fiat {
			code = strings.ToUpper(code)
			if v, ok := rates[code]; ok && v > 0 {
				t.Quotes = append(t.Quotes, Quote{Symbol: code, Price: v})
			}
		}
	}
	for _, code := range p.Crypto {
		code = strings.ToUpper(code)
		price, err := spot(ctx, client, code, base)
		if err != nil {
			// One asset that will not answer is not a reason to drop the
			// rest of the line.
			continue
		}
		t.Quotes = append(t.Quotes, Quote{Symbol: code, Price: price})
	}
	if len(t.Quotes) == 0 {
		return Table{}, fmt.Errorf("no quote could be read")
	}
	return t, nil
}

func (p *Poller) fiat(ctx context.Context, client *http.Client, base string) (map[string]float64, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fiatEndpoint+base, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", fiatEndpoint+base, resp.Status)
	}
	var doc struct {
		Result string             `json:"result"`
		Rates  map[string]float64 `json:"rates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, err
	}
	if doc.Result != "success" {
		return nil, fmt.Errorf("rates: %q", doc.Result)
	}
	return doc.Rates, nil
}

func spot(ctx context.Context, client *http.Client, asset, base string) (float64, error) {
	url := cryptoEndpoint + asset + "-" + base + "/spot"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("%s: %s", url, resp.Status)
	}
	var doc struct {
		Data struct {
			Amount string `json:"amount"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return 0, err
	}
	return strconv.ParseFloat(doc.Data.Amount, 64)
}

// Line is the quotes as one string: "EUR 0.86 · BTC 78k". Prices are
// written the way a glance reads them, not the way a ledger does.
func (t Table) Line() string {
	if len(t.Quotes) == 0 {
		return ""
	}
	parts := make([]string, 0, len(t.Quotes))
	for _, q := range t.Quotes {
		parts = append(parts, q.Symbol+" "+short(q.Price))
	}
	return strings.Join(parts, " · ")
}

// short writes as many digits as the number is worth at a glance: no
// kopecks on a bitcoin, and no bare zero on a token worth a fraction.
func short(v float64) string {
	switch {
	case v >= 10000:
		return strconv.FormatFloat(v/1000, 'f', 0, 64) + "k"
	case v >= 10:
		return strconv.FormatFloat(v, 'f', 0, 64)
	case v >= 0.5:
		return strconv.FormatFloat(v, 'f', 2, 64)
	}
	return strconv.FormatFloat(v, 'f', 4, 64)
}
