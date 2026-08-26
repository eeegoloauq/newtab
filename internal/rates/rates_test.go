package rates

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLineReadsAtAGlance(t *testing.T) {
	got := Table{Base: "USD", Quotes: []Quote{
		{"EUR", 0.857}, {"RUB", 84.06}, {"BTC", 78038.83}, {"DOGE", 0.1234},
	}}.Line()
	// Two decimals below ten, none in the hundreds, thousands as k: the
	// point is a glance, not a ledger.
	want := "EUR 0.86 · RUB 84 · BTC 78k · DOGE 0.1234"
	if got != want {
		t.Fatalf("line = %q, want %q", got, want)
	}
	if (Table{}).Line() != "" {
		t.Fatal("an empty table must render nothing at all")
	}
}

func TestFetchTakesWhatItAskedForInOrder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v6/latest/USD":
			w.Write([]byte(`{"result":"success","rates":{"EUR":0.86,"RUB":84.1,"JPY":159.2}}`))
		case r.URL.Path == "/v2/prices/BTC-USD/spot":
			w.Write([]byte(`{"data":{"amount":"78038.83"}}`))
		default:
			// An asset nobody quotes must not take the line down with it.
			http.Error(w, "no", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := &Poller{Base: "USD", Fiat: []string{"eur", "RUB"}, Crypto: []string{"BTC", "NOPE"}}
	client := &http.Client{Transport: rewrite{srv.URL}}
	table, err := p.fetch(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	if table.Line() != "EUR 0.86 · RUB 84 · BTC 78k" {
		t.Fatalf("line = %q", table.Line())
	}
}

func TestFetchFailsWhenNothingAnswered(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "no", http.StatusInternalServerError)
	}))
	defer srv.Close()
	p := &Poller{Base: "USD", Crypto: []string{"BTC"}}
	if _, err := p.fetch(context.Background(), &http.Client{Transport: rewrite{srv.URL}}); err == nil {
		t.Fatal("a table with no quotes in it was reported as a reading")
	}
}

func TestTableBeforeAPollIsEmpty(t *testing.T) {
	var p Poller
	if p.Table().Line() != "" {
		t.Fatal("an unpolled poller quoted a price")
	}
	if p.Every != 0 {
		t.Fatalf("Every = %s", p.Every)
	}
}

type rewrite struct{ base string }

func (rw rewrite) RoundTrip(r *http.Request) (*http.Response, error) {
	req, err := http.NewRequest(r.Method, rw.base+r.URL.Path, nil)
	if err != nil {
		return nil, err
	}
	req.Header = r.Header
	c := &http.Client{Timeout: 5 * time.Second}
	return c.Do(req)
}
