package icons

import (
	"net/http"
	"reflect"
	"testing"
)

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
	if got := Slug("!!!"); got != "e84c538e" {
		t.Fatalf("Slug(%q) = %q: the digest must not drift, it names a file", "!!!", got)
	}
	if Slug("!!!") == Slug("???") {
		t.Fatal("two symbol-only names collided")
	}
}

// A hand-built transport ignores HTTPS_PROXY unless it is told not to,
// and the fetch quietly went direct. On a network that blocks a site,
// that is the difference between having its icon and not.
func TestClientHonoursProxyEnvironment(t *testing.T) {
	tr, ok := newClient(0).Transport.(*http.Transport)
	if !ok || tr.Proxy == nil {
		t.Fatal("the icon client would ignore HTTP_PROXY and HTTPS_PROXY")
	}
	// Comparing the function itself rather than its behaviour: the
	// environment is read once per process, so setting it here would come
	// too late to observe.
	if reflect.ValueOf(tr.Proxy).Pointer() != reflect.ValueOf(http.ProxyFromEnvironment).Pointer() {
		t.Fatal("the proxy hook is not ProxyFromEnvironment")
	}
}
