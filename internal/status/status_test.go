package status

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const doc = `{"version":1,"checks":[
 {"name":"Photos","url":"https://photos.example.com/","remote_addr":"198.51.100.7:443",
  "status":"up","muted":false,"last_probe":{"duration_ms":23},"incident":null},
 {"name":"Git","url":"http://198.51.100.12:3000","remote_addr":"198.51.100.12:3000",
  "status":"down","muted":false,"last_probe":{"duration_ms":0},"incident":{"duration_ms":1200000}},
 {"name":"Quiet","url":"https://quiet.example.com/","remote_addr":"198.51.100.9:443",
  "status":"down","muted":true,"last_probe":{"duration_ms":0},"incident":{"duration_ms":60000}}
]}`

func serve(t *testing.T, body string) *Poller {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &Poller{URL: srv.URL, Every: time.Hour}
}

func TestFetchReadsTheMonitorDocument(t *testing.T) {
	p := serve(t, doc)
	snap, err := p.fetch(context.Background(), &http.Client{Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	// A link that names no check is matched by the host it points at,
	// which is what makes a config with no check names work at all.
	c, ok := snap.Lookup("", "https://photos.example.com/albums")
	if !ok || !c.Up || c.LatencyMS != 23 {
		t.Fatalf("photos = %+v ok=%v", c, ok)
	}
	// An explicit name wins, and it is how two checks on one host are
	// told apart.
	if c, ok := snap.Lookup("Git", "https://elsewhere.example/"); !ok || c.Up || c.Down != 20*time.Minute {
		t.Fatalf("git = %+v ok=%v", c, ok)
	}
	if _, ok := snap.Lookup("", "https://nothing.example/"); ok {
		t.Fatal("a link with no check must report nothing, not a guess")
	}
}

// A muted check is silenced on purpose. Reporting it as up would be a
// lie, and reporting it as down would be the noise muting removed.
func TestMutedChecksAreCarriedThrough(t *testing.T) {
	p := serve(t, doc)
	snap, err := p.fetch(context.Background(), &http.Client{Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	c, ok := snap.Lookup("Quiet", "")
	if !ok || !c.Muted {
		t.Fatalf("quiet = %+v ok=%v", c, ok)
	}
}

// The monitor being unreachable must cost the page nothing: an empty
// snapshot renders rows with nothing to say.
func TestSnapshotBeforeTheFirstPollIsEmptyRatherThanNil(t *testing.T) {
	var p Poller
	if _, ok := p.Snapshot().Lookup("", "https://anything.example/"); ok {
		t.Fatal("an unpolled poller claimed to know something")
	}
}

func TestRunKeepsTheLastGoodAnswerWhenAPollFails(t *testing.T) {
	fail := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail {
			http.Error(w, "no", http.StatusInternalServerError)
			return
		}
		w.Write([]byte(doc))
	}))
	defer srv.Close()
	p := &Poller{URL: srv.URL, Every: 10 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	go p.Run(ctx)
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, ok := p.Snapshot().Lookup("Git", ""); ok {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("the first poll never landed")
		}
		time.Sleep(5 * time.Millisecond)
	}
	fail = true
	time.Sleep(50 * time.Millisecond)
	cancel()
	if _, ok := p.Snapshot().Lookup("Git", ""); !ok {
		t.Fatal("a failed poll blanked the page; a monitor restart is not an outage of everything")
	}
}
