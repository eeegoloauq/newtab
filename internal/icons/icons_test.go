package icons

import "testing"

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Nginx Proxy Manager": "nginx-proxy-manager",
		"REG.RU":              "reg-ru",
		"Last.fm":             "last-fm",
	}
	for name, want := range cases {
		if got := Slug(name); got != want {
			t.Fatalf("Slug(%q) = %q, want %q", name, got, want)
		}
	}
}

// Cyrillic names once all reduced to the empty slug, so every one of them
// wrote to the same file and rendered blank.
func TestSlugKeepsCyrillicNamesApart(t *testing.T) {
	a, b := Slug("Кинопоиск"), Slug("Госуслуги")
	if a == "" || b == "" {
		t.Fatalf("empty slug: %q %q", a, b)
	}
	if a == b {
		t.Fatalf("two names collided on %q", a)
	}
	if got := Slug("Коты Коломны"); got != "koty-kolomny" {
		t.Fatalf("Slug = %q, want koty-kolomny", got)
	}
}

func TestSlugOfSymbolsOnlyIsStable(t *testing.T) {
	if Slug("!!!") == "" || Slug("!!!") != Slug("!!!") {
		t.Fatal("a name of symbols must still get a stable, non-empty slug")
	}
}
