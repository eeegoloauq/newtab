package weather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The codes are the part of this that will look wrong to the next
// reader: they are a standard, and the folding into six drawings is a
// decision, not an oversight.
func TestCodesFoldIntoSomethingDrawable(t *testing.T) {
	cases := map[int]Sky{
		0: Clear, 1: Clear,
		2: Cloudy, 3: Cloudy,
		45: Fog, 48: Fog,
		51: Rain, 61: Rain, 65: Rain, 80: Rain, 82: Rain,
		71: Snow, 75: Snow, 86: Snow,
		95: Storm, 99: Storm,
	}
	for code, want := range cases {
		if got := skyOf(code); got != want {
			t.Errorf("code %d = %q, want %q", code, got, want)
		}
	}
	// Anything unknown has to draw something rather than nothing.
	if skyOf(12345) == "" {
		t.Error("an unknown code left the page with no glyph")
	}
}

func TestFetchReadsTheCurrentBlock(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`{"current":{"temperature_2m":11.6,"weather_code":61,"is_day":0}}`))
	}))
	defer srv.Close()
	p := &Poller{Latitude: 59.9386, Longitude: 30.3141}
	// The endpoint is a constant, so the test points the client at the
	// stub by rewriting the transport rather than the URL.
	client := &http.Client{Transport: rewrite{srv.URL}}
	n, err := p.fetch(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	if n.Temperature != 12 || n.Sky != Rain || !n.Night || !n.OK {
		t.Fatalf("now = %+v", n)
	}
	// Coordinates are rounded before they leave: the service has no
	// business knowing which building asked.
	if gotQuery == "" || !contains(gotQuery, "latitude=59.94") {
		t.Fatalf("query = %q", gotQuery)
	}
}

func TestFahrenheitIsAskedForRatherThanConverted(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`{"current":{"temperature_2m":53.1,"weather_code":0,"is_day":1}}`))
	}))
	defer srv.Close()
	p := &Poller{Latitude: 1, Longitude: 1, Fahrenheit: true}
	if _, err := p.fetch(context.Background(), &http.Client{Transport: rewrite{srv.URL}}); err != nil {
		t.Fatal(err)
	}
	if !contains(gotQuery, "temperature_unit=fahrenheit") {
		t.Fatalf("query = %q", gotQuery)
	}
}

func TestNowBeforeAPollSaysNothing(t *testing.T) {
	var p Poller
	if p.Now().OK {
		t.Fatal("an unpolled poller claimed to know the weather")
	}
	// Run's default cadence lives with Run; this only guards that an
	// unset Every does not mean "poll continuously".
	if p.Every != 0 {
		t.Fatalf("Every = %s", p.Every)
	}
}

// rewrite sends every request to the stub, keeping path and query.
type rewrite struct{ base string }

func (rw rewrite) RoundTrip(r *http.Request) (*http.Response, error) {
	u := *r.URL
	stub, err := http.NewRequest(r.Method, rw.base+u.Path+"?"+u.RawQuery, nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultTransport.RoundTrip(stub)
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
