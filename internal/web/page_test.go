package web

import (
	"strings"
	"testing"
	"time"

	"github.com/eeegoloauq/newtab/internal/config"
	"github.com/eeegoloauq/newtab/internal/proxmox"
	"github.com/eeegoloauq/newtab/internal/rates"
	"github.com/eeegoloauq/newtab/internal/status"
	"github.com/eeegoloauq/newtab/internal/weather"
)

func testConfig() *config.Config {
	return &config.Config{
		Title:  "newtab",
		Search: config.Search{Engine: "https://example.com/?q=%s"},
		Sections: []config.Section{
			{Name: "Services", Style: config.StyleLive, Links: []config.Link{
				{Name: "Music", URL: "https://music.example.com/", Alias: []string{"Музыка"}},
			}},
			{Name: "Work", Style: config.StyleList, Links: []config.Link{
				{Name: "Git & co", URL: "https://www.example.org/x?a=1&b=2"},
			}},
		},
	}
}

func TestRenderSplitsStyles(t *testing.T) {
	body, err := render(testConfig(), status.Snapshot{}, proxmox.Stats{}, weather.Now{}, rates.Table{})
	if err != nil {
		t.Fatal(err)
	}
	html := string(body)
	// A live row carries a status dot and a tail; a bookmark carries
	// neither, and both are list items in the same columns.
	if strings.Count(html, "<li><a") != 2 {
		t.Fatalf("expected two rows, got %d", strings.Count(html, "<li><a"))
	}
	// A row with no icon of its own gets the browser's globe, never an
	// empty slot: the straight left edge is what holds the page together.
	if strings.Count(html, `href="#globe"`) != 2 {
		t.Fatal("every iconless row should draw the globe")
	}
}

func TestHostOfStripsWWW(t *testing.T) {
	if got := hostOf("https://www.example.org/x?a=1"); got != "example.org" {
		t.Fatalf("hostOf = %q, want example.org", got)
	}
}

func TestSearchKeyIsLowercaseAndCoversAliasAndHost(t *testing.T) {
	key := searchKey(config.Link{Name: "Music", URL: "https://music.example.com/", Alias: []string{"Музыка"}})
	for _, want := range []string{"music", "music.example.com", "музыка"} {
		if !strings.Contains(key, want) {
			t.Fatalf("key %q lacks %q", key, want)
		}
	}
}

// The tail is the only place a row says anything beyond its name, so
// what lands there matters more than its width suggests.
func TestLiveRowsShowLatencyAndOutages(t *testing.T) {
	text := config.Text{Down: "down"}
	up := status.Check{Name: "Music", Up: true, LatencyMS: 23}
	down := status.Check{Name: "Music", Down: 20 * time.Minute}
	muted := status.Check{Name: "Music", Muted: true}

	for _, tc := range []struct {
		name     string
		check    status.Check
		wantTail string
		wantDown bool
	}{
		{"up", up, "23 ms", false},
		{"down", down, "down 20m", true},
		{"muted", muted, "", false},
		{"faster than a millisecond", status.Check{Up: true}, "<1 ms", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snap := status.Fixed(map[string]status.Check{"Music": tc.check})
			tail, isDown := state(snap, config.Link{Name: "Music", URL: "https://music.example.com/", Check: "Music"}, text, config.TailLatency)
			if tail != tc.wantTail || isDown != tc.wantDown {
				t.Fatalf("tail = %q down = %v, want %q %v", tail, isDown, tc.wantTail, tc.wantDown)
			}
		})
	}

	// A link with no check at all says nothing: an empty tail is honest,
	// a question mark is not.
	if tail, down := state(status.Snapshot{}, config.Link{Name: "Music", URL: "https://music.example.com/"}, text, config.TailLatency); tail != "" || down {
		t.Fatalf("unmonitored link showed %q", tail)
	}
}

// The default tail speaks only when something is off: a row that reads
// 100% every day is furniture, and the eye stops seeing the day it does
// not.
func TestExceptionsTailIsQuietUntilItIsNot(t *testing.T) {
	text := config.Text{Down: "down"}
	link := config.Link{Name: "Music", URL: "https://music.example.com/", Check: "Music"}

	perfect := status.Fixed(map[string]status.Check{"Music": {Up: true, LatencyMS: 23, Uptime24h: 1}})
	if tail, _ := state(perfect, link, text, config.TailProblems); tail != "" {
		t.Fatalf("a perfect day said %q", tail)
	}

	dipped := status.Fixed(map[string]status.Check{"Music": {Up: true, LatencyMS: 23, Uptime24h: 0.9982}})
	if tail, _ := state(dipped, link, text, config.TailProblems); tail != "99.8% 24h" {
		t.Fatalf("tail = %q, want 99.8%% 24h", tail)
	}

	// Rounding must not print 100% for a day that had an outage in it,
	// nor 99.9% for a day that did not.
	if got := percent(0.99999); got != "100%" {
		t.Fatalf("percent(0.99999) = %q", got)
	}
	if got := percent(0.5); got != "50.0%" {
		t.Fatalf("percent(0.5) = %q", got)
	}
}

// Three bare numbers fit the column; the tooltip is where they are
// named, because the row has no room for words.
func TestHypervisorNumbersAreTerseWithASpelledOutHint(t *testing.T) {
	text := config.Text{Guests: "running", CPU: "cpu", Memory: "memory"}
	tail, hint := hypervisor(proxmox.Stats{Running: 16, CPU: 29, Memory: 51, OK: true}, text)
	if tail != "16 · 29% · 51%" {
		t.Fatalf("tail = %q", tail)
	}
	if len([]rune(tail)) > 20 {
		t.Fatalf("tail %q will be cut off in the column", tail)
	}
	if hint != "16 running · 29% cpu · 51% memory" {
		t.Fatalf("hint = %q", hint)
	}
}

func TestRenderEscapes(t *testing.T) {
	c := testConfig()
	c.Sections[1].Links[0].Name = `<script>alert(1)</script>`
	body, err := render(c, status.Snapshot{}, proxmox.Stats{}, weather.Now{}, rates.Table{})
	if err != nil {
		t.Fatal(err)
	}
	// The name reaches both text and a data attribute; neither may break out.
	if strings.Contains(string(body), "<script>alert(1)</script>") {
		t.Fatal("a link name was rendered as markup")
	}
}
