package proxmox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const resourcesJSON = `{"data":[
 {"type":"node","node":"pve","cpu":0.207,"mem":8286437376,"maxmem":16538959872},
 {"type":"lxc","status":"running"},
 {"type":"lxc","status":"running"},
 {"type":"lxc","status":"stopped"},
 {"type":"qemu","status":"running"},
 {"type":"storage","status":"available"}
]}`

func TestFetchReadsTheThreeNumbers(t *testing.T) {
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.Path
		w.Write([]byte(resourcesJSON))
	}))
	defer srv.Close()

	p := &Poller{URL: srv.URL + "/", Token: "user@pve!id=secret"}
	s, err := p.fetch(context.Background(), &http.Client{Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	// Stopped guests are not what the number is about, and storage rows
	// are not guests at all.
	if s.Running != 3 {
		t.Errorf("running = %d, want 3", s.Running)
	}
	if s.CPU != 21 {
		t.Errorf("cpu = %d%%, want 21%%", s.CPU)
	}
	if s.Memory != 50 {
		t.Errorf("memory = %d%%, want 50%%", s.Memory)
	}
	if !s.OK {
		t.Error("a successful poll must mark itself usable")
	}
	// The token goes in a header as a token, and a trailing slash on the
	// configured URL must not double up in the path.
	if gotAuth != "PVEAPIToken=user@pve!id=secret" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotPath != "/api2/json/cluster/resources" {
		t.Errorf("path = %q", gotPath)
	}
}

// Before the first answer the row must say nothing: zeroes would read as
// an idle hypervisor, which is a different claim from "not known yet".
func TestStatsBeforeAPollSayNothing(t *testing.T) {
	var p Poller
	if tail := p.Stats().Tail(); tail != "" {
		t.Fatalf("unpolled stats said %q", tail)
	}
}

func TestTailFitsOneColumn(t *testing.T) {
	got := Stats{Running: 16, CPU: 29, Memory: 51, OK: true}.Tail()
	if got != "16 · 29% · 51%" {
		t.Fatalf("tail = %q", got)
	}
	// The slot is about twenty characters wide before it truncates.
	if len([]rune(got)) > 20 {
		t.Fatalf("tail %q is %d characters and will be cut off", got, len([]rune(got)))
	}
}

func TestFetchRejectsAnErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "no ticket", http.StatusUnauthorized)
	}))
	defer srv.Close()
	p := &Poller{URL: srv.URL, Token: "bad"}
	if _, err := p.fetch(context.Background(), &http.Client{Timeout: time.Second}); err == nil {
		t.Fatal("a 401 was read as numbers")
	}
}
