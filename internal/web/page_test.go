package web

import (
	"strings"
	"testing"

	"github.com/eeegoloauq/newtab/internal/config"
)

func testConfig() *config.Config {
	return &config.Config{
		Title:  "newtab",
		Search: config.Search{Engine: "https://example.com/?q=%s"},
		Sections: []config.Section{
			{Name: "Services", Style: config.StyleCards, Links: []config.Link{
				{Name: "Music", URL: "https://music.example.com/", Alias: []string{"Музыка"}},
			}},
			{Name: "Work", Style: config.StyleList, Links: []config.Link{
				{Name: "Git & co", URL: "https://www.example.org/x?a=1&b=2"},
			}},
		},
	}
}

func TestRenderSplitsStyles(t *testing.T) {
	body, err := render(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	html := string(body)
	if !strings.Contains(html, `class="card"`) || !strings.Contains(html, "<li>") {
		t.Fatal("both a card and a list item were expected")
	}
	// The host is shown on cards and must be the bare one.
	if !strings.Contains(html, ">music.example.com<") {
		t.Fatal("card host missing")
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

func TestRenderEscapes(t *testing.T) {
	c := testConfig()
	c.Sections[1].Links[0].Name = `<script>alert(1)</script>`
	body, err := render(c)
	if err != nil {
		t.Fatal(err)
	}
	// The name reaches both text and a data attribute; neither may break out.
	if strings.Contains(string(body), "<script>alert(1)</script>") {
		t.Fatal("a link name was rendered as markup")
	}
}
