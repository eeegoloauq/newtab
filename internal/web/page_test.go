package web

import (
	"strings"
	"testing"
	"time"

	"github.com/eeegoloauq/newtab/internal/config"
	"github.com/eeegoloauq/newtab/internal/status"
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
	body, err := render(testConfig(), status.Snapshot{})
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
			tail, isDown := state(snap, config.Link{Name: "Music", URL: "https://music.example.com/", Check: "Music"}, text)
			if tail != tc.wantTail || isDown != tc.wantDown {
				t.Fatalf("tail = %q down = %v, want %q %v", tail, isDown, tc.wantTail, tc.wantDown)
			}
		})
	}

	// A link with no check at all says nothing: an empty tail is honest,
	// a question mark is not.
	if tail, down := state(status.Snapshot{}, config.Link{Name: "Music", URL: "https://music.example.com/"}, text); tail != "" || down {
		t.Fatalf("unmonitored link showed %q", tail)
	}
}

func TestRenderEscapes(t *testing.T) {
	c := testConfig()
	c.Sections[1].Links[0].Name = `<script>alert(1)</script>`
	body, err := render(c, status.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	// The name reaches both text and a data attribute; neither may break out.
	if strings.Contains(string(body), "<script>alert(1)</script>") {
		t.Fatal("a link name was rendered as markup")
	}
}
