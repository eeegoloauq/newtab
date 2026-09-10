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

func TestHostOfStripsWWWAndLowercases(t *testing.T) {
	// The filter compares a host against a query that is already
	// lowercase, and a URL keeps whatever case it was typed in — so the
	// case has to go before the www., not after it.
	for _, raw := range []string{"https://www.example.org/x?a=1", "https://WWW.Example.ORG/"} {
		if got := hostOf(raw); got != "example.org" {
			t.Fatalf("hostOf(%q) = %q, want example.org", raw, got)
		}
	}
}

// The filter scores a hit on a name, on an alias and on a host
// differently, so the three reach it as three values. Joined into one
// string they were indistinguishable, which is how every row on a
// one-domain page came to answer to that domain.
func TestSearchKeysAreLowercaseAndSeparate(t *testing.T) {
	if got := searchKey("Music"); got != "music" {
		t.Fatalf("searchKey = %q, want music", got)
	}
	if got := aliasKey([]string{"Музыка", "Sound"}); got != "музыка|sound" {
		t.Fatalf("aliasKey = %q, want музыка|sound", got)
	}
	if got := aliasKey(nil); got != "" {
		t.Fatalf("aliasKey(nil) = %q, want empty", got)
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

// The browser paints the canvas from this before it has read a line of
// CSS: dark on a light page is a black flash, and the other way round is
// a white one.
func TestColourSchemeFollowsTheConfiguredBackground(t *testing.T) {
	for background, want := range map[string]string{
		"":         "dark",
		"#141312":  "dark",
		"#f6f3ec":  "light",
		"nonsense": "dark",
	} {
		if got := scheme(background); got != want {
			t.Errorf("scheme(%q) = %q, want %q", background, got, want)
		}
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

// Columns are packed by height, so a section's place in the document is not
// its place in the config. This attribute carries the config order the
// filter needs: between two rows that match a query equally well, the one
// the operator wrote first wins.
func TestSectionsCarryTheirConfigOrder(t *testing.T) {
	body, err := render(testConfig(), status.Snapshot{}, proxmox.Stats{}, weather.Now{}, rates.Table{})
	if err != nil {
		t.Fatal(err)
	}
	html := string(body)
	services := strings.Index(html, `style="order:0"`)
	work := strings.Index(html, `style="order:1"`)
	if services < 0 || work < 0 {
		t.Fatalf("sections are missing their config order:\n%s", html)
	}
	// The results list has to exist in the markup for the script to move
	// rows into, and the stylesheet has to put the columns away while it
	// is showing.
	if !strings.Contains(html, `<ul class="hits" id="hits" role="list">`) {
		t.Error("the page has no results list for the filter to fill")
	}
	if !strings.Contains(html, ".filtering .col") {
		t.Error("the stylesheet no longer puts the columns away while filtering")
	}
}
