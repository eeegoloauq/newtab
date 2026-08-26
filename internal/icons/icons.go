// Package icons fetches a site's own favicon once and stores it on disk.
//
// It goes to the site itself, never to an icon service: asking a search
// engine for 47 favicons hands it the whole list of places you go. The
// fetch happens when a link is added, not while a page is being served —
// the start page must never wait on the network.
package icons

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// maxIcon caps what we will store. A favicon above this is not a favicon,
// it is a mistake on the other end.
const maxIcon = 256 << 10

// maxCandidates is how many declared icons we will try before falling
// back to the well-known paths.
const maxCandidates = 8

// browserUA is not decoration: a plain Go user agent gets a 403 from a
// good share of the sites people actually bookmark.
const browserUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0 Safari/537.36"

// newClient trusts any certificate. An icon is a picture, not a secret,
// and half the boxes on a home LAN answer https with a self-signed cert —
// verifying here would mean no icon for the hypervisor and the router
// forever.
func newClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

// browserHeaders are what a real browser sends. Sites behind a bot
// filter answer 403 to anything less, and their icon is exactly the one
// you want most, because you visit them daily.
func browserHeaders(req *http.Request) {
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,image/avif,image/webp,image/apng,*/*;q=0.8")
	// No language preference of our own: asking for one gets a localised
	// page, and sometimes a localised icon.
	req.Header.Set("Accept-Language", "*")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
}

var slugUnsafe = regexp.MustCompile(`[^a-z0-9]+`)

// cyrillic is transliterated rather than dropped. Dropping it turned
// every Russian-named link into the same empty slug, so they all shared
// one file and every one of them rendered blank.
var cyrillic = strings.NewReplacer(
	"а", "a", "б", "b", "в", "v", "г", "g", "д", "d", "е", "e", "ё", "e",
	"ж", "zh", "з", "z", "и", "i", "й", "y", "к", "k", "л", "l", "м", "m",
	"н", "n", "о", "o", "п", "p", "р", "r", "с", "s", "т", "t", "у", "u",
	"ф", "f", "х", "h", "ц", "c", "ч", "ch", "ш", "sh", "щ", "sch",
	"ъ", "", "ы", "y", "ь", "", "э", "e", "ю", "yu", "я", "ya",
)

// Slug is the file name an icon is stored under and the last path element
// it is served from. Link names are unique by config validation, and a
// slug must stay unique too, so a name that survives transliteration as
// nothing gets a digest instead of an empty string.
func Slug(name string) string {
	if name == "" {
		return ""
	}
	s := slugUnsafe.ReplaceAllString(cyrillic.Replace(strings.ToLower(name)), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		sum := sha256.Sum256([]byte(name))
		return hex.EncodeToString(sum[:4])
	}
	return s
}

// Store is a directory of fetched icons.
type Store struct {
	Dir string
	// Timeout bounds one HTTP request. Zero means defaultTimeout; it is a
	// field so a test can make a hang finish in milliseconds instead of
	// holding the suite for the real fifteen seconds.
	Timeout time.Duration
}

const defaultTimeout = 15 * time.Second

// Path returns the stored file for a link, or "" if there is none. The
// extension is unknown up front, so this globs.
func (s Store) Path(name string) string {
	if s.Dir == "" {
		return ""
	}
	matches, err := filepath.Glob(filepath.Join(s.Dir, Slug(name)+".*"))
	if err != nil || len(matches) == 0 {
		return ""
	}
	return matches[0]
}

// Icon returns the stored file to draw for a link, or "" if there is
// none worth drawing. It checks every candidate rather than the first
// the glob returns: a leftover HTML file named .ico sitting next to a
// good .png would otherwise hide the good one.
//
// A file written before the fetcher learned to sniff its downloads can
// be an error page with an image name, and it renders as a torn square
// rather than as nothing — worse than having no icon at all.
func (s Store) Icon(name string) string {
	if s.Dir == "" || Slug(name) == "" {
		return ""
	}
	matches, err := filepath.Glob(filepath.Join(s.Dir, Slug(name)+".*"))
	if err != nil {
		return ""
	}
	for _, m := range matches {
		if isImageFile(m) {
			return m
		}
	}
	return ""
}

// IconBySlug is Icon for a slug that has already been derived, which is
// what an HTTP path carries.
func (s Store) IconBySlug(slug string) string {
	if s.Dir == "" || slug == "" || Slug(slug) != slug {
		return ""
	}
	matches, err := filepath.Glob(filepath.Join(s.Dir, slug+".*"))
	if err != nil {
		return ""
	}
	for _, m := range matches {
		if isImageFile(m) {
			return m
		}
	}
	return ""
}

// Valid reports whether there is an icon worth drawing for this link.
func (s Store) Valid(name string) bool { return s.Icon(name) != "" }

func isImageFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, 512)
	n, _ := f.Read(head)
	head = head[:n]
	return n > 0 && (strings.HasPrefix(http.DetectContentType(head), "image/") || isSVG(head))
}

// Fetch downloads the icon for pageURL and writes it into the store.
// It reports the file written. An error here is never fatal to the page:
// the caller falls back to a monogram.
func (s Store) Fetch(ctx context.Context, name, pageURL string) (string, error) {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return "", err
	}
	client := newClient(s.Timeout)
	candidates, err := candidates(ctx, client, pageURL)
	if err != nil {
		return "", err
	}
	var refused []string
	for _, c := range candidates {
		body, ext, err := download(ctx, client, c)
		if err != nil {
			// Keep why: "no icon found" tells the operator nothing about
			// whether to retry, drop the link or place a file by hand.
			refused = append(refused, err.Error())
			continue
		}
		// Write first, delete after. Deleting first meant a flaky network
		// on a -force run wiped a working icon and put nothing back.
		out := filepath.Join(s.Dir, Slug(name)+ext)
		tmp, err := os.CreateTemp(s.Dir, Slug(name)+".*.part")
		if err != nil {
			return "", err
		}
		if _, err := tmp.Write(body); err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
			return "", err
		}
		tmp.Close()
		if err := os.Rename(tmp.Name(), out); err != nil {
			os.Remove(tmp.Name())
			return "", err
		}
		// A site that moved from .ico to .svg would otherwise keep both,
		// and Path would pick whichever the glob returned first.
		for _, old := range must(filepath.Glob(filepath.Join(s.Dir, Slug(name)+".*"))) {
			if old != out {
				os.Remove(old)
			}
		}
		return out, nil
	}
	if len(refused) == 0 {
		return "", fmt.Errorf("the page declares no icon and has no /favicon.ico")
	}
	return "", fmt.Errorf("%s", strings.Join(refused, "; "))
}

func must(m []string, _ error) []string { return m }

// candidates lists icon URLs for a page, best first: what the page
// declares, largest first, then the well-known path every browser tries.
func candidates(ctx context.Context, client *http.Client, pageURL string) ([]string, error) {
	base, err := url.Parse(pageURL)
	if err != nil {
		return nil, err
	}
	wellKnown := []string{
		base.ResolveReference(&url.URL{Path: "/favicon.ico"}).String(),
		// Sites that dropped the .ico usually still ship this one.
		base.ResolveReference(&url.URL{Path: "/apple-touch-icon.png"}).String(),
		base.ResolveReference(&url.URL{Path: "/favicon.svg"}).String(),
	}

	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return nil, err
	}
	browserHeaders(req)
	resp, err := client.Do(req)
	if err != nil {
		// Some CDNs drop a request carrying Sec-Fetch-* from a client
		// they do not recognise. Ask once more as plainly as possible.
		plain, perr := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
		if perr != nil {
			return wellKnown, nil
		}
		plain.Header.Set("User-Agent", browserUA)
		resp, err = client.Do(plain)
	}
	if err != nil {
		// The page refused us; the well-known paths are often still served.
		return wellKnown, nil
	}
	defer resp.Body.Close()
	// A 500 body can still contain a <link rel=icon> from the site's
	// error template, and that is not the icon of the page we asked for.
	if resp.StatusCode != http.StatusOK {
		return wellKnown, nil
	}
	doc, err := html.Parse(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return wellKnown, nil
	}
	// resp.Request.URL, not base: follow where the redirects ended up.
	found := declared(doc, resp.Request.URL)
	// A page is free to declare a hundred icons. Trying them all would
	// spend the whole run on one link.
	if len(found) > maxCandidates {
		found = found[:maxCandidates]
	}
	return append(found, wellKnown...), nil
}

type declaredIcon struct {
	url  string
	size int
}

func declared(doc *html.Node, base *url.URL) []string {
	var out []declaredIcon
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "link" {
			var rel, href, sizes string
			for _, a := range n.Attr {
				switch strings.ToLower(a.Key) {
				case "rel":
					rel = strings.ToLower(a.Val)
				case "href":
					href = a.Val
				case "sizes":
					sizes = a.Val
				}
			}
			if href != "" && (strings.Contains(rel, "icon")) {
				if u, err := base.Parse(href); err == nil {
					out = append(out, declaredIcon{u.String(), parseSize(sizes, rel)})
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	// Largest wins: a 16px icon is a blurred smudge on a HiDPI screen.
	sort.SliceStable(out, func(i, j int) bool { return out[i].size > out[j].size })
	urls := make([]string, 0, len(out))
	for _, i := range out {
		urls = append(urls, i.url)
	}
	return urls
}

func parseSize(sizes, rel string) int {
	// "any" means SVG, which beats every raster size there is.
	if strings.Contains(strings.ToLower(sizes), "any") {
		return 1 << 16
	}
	if n, _, ok := strings.Cut(strings.ToLower(sizes), "x"); ok {
		if v, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
			return v
		}
	}
	// An apple-touch-icon carries no sizes attribute more often than not,
	// and is 180px by convention.
	if strings.Contains(rel, "apple") {
		return 180
	}
	return 0
}

var extByType = map[string]string{
	"image/svg+xml":            ".svg",
	"image/png":                ".png",
	"image/jpeg":               ".jpg",
	"image/webp":               ".webp",
	"image/gif":                ".gif",
	"image/x-icon":             ".ico",
	"image/vnd.microsoft.icon": ".ico",
	"image/ico":                ".ico",
}

func download(ctx context.Context, client *http.Client, raw string) ([]byte, string, error) {
	// A data: URI is the icon, already delivered with the page. Nothing
	// to fetch, and no reason to skip a site that inlines its icon.
	if strings.HasPrefix(raw, "data:") {
		return decodeData(raw)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", raw, nil)
	if err != nil {
		return nil, "", err
	}
	browserHeaders(req)
	req.Header.Set("Sec-Fetch-Dest", "image")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("%s: %s", raw, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxIcon+1))
	if err != nil {
		return nil, "", err
	}
	if len(body) == 0 || len(body) > maxIcon {
		return nil, "", fmt.Errorf("%s: %d bytes", raw, len(body))
	}
	ct, _, _ := strings.Cut(resp.Header.Get("Content-Type"), ";")
	ext, ok := extByType[strings.TrimSpace(strings.ToLower(ct))]
	if !ok {
		// Some servers send text/plain for .ico. Trust the path instead,
		// but only for extensions we would have accepted anyway.
		ext = strings.ToLower(filepath.Ext(strings.SplitN(raw, "?", 2)[0]))
		if _, known := contentTypeByExt[ext]; !known {
			return nil, "", fmt.Errorf("%s: unusable type %q", raw, ct)
		}
	}
	// Serving an HTML error page with 200 and an image content type is a
	// common way for a site to "have" a favicon. Sniff the bytes: the
	// headers and the file name have both already lied by this point.
	if sniffed := http.DetectContentType(body); !strings.HasPrefix(sniffed, "image/") && !isSVG(body) {
		return nil, "", fmt.Errorf("%s: %s, not an image", raw, sniffed)
	}
	return body, ext, nil
}

func decodeData(raw string) ([]byte, string, error) {
	meta, payload, ok := strings.Cut(strings.TrimPrefix(raw, "data:"), ",")
	if !ok {
		return nil, "", fmt.Errorf("malformed data: URI")
	}
	mime, _, _ := strings.Cut(meta, ";")
	ext, known := extByType[strings.TrimSpace(strings.ToLower(mime))]
	if !known {
		return nil, "", fmt.Errorf("data: URI of type %q", mime)
	}
	var body []byte
	var err error
	if strings.Contains(meta, "base64") {
		body, err = base64.StdEncoding.DecodeString(payload)
	} else {
		var unescaped string
		// PathUnescape, not QueryUnescape: the latter turns a literal
		// plus into a space and corrupts the payload.
		unescaped, err = url.PathUnescape(payload)
		body = []byte(unescaped)
	}
	if err != nil || len(body) == 0 || len(body) > maxIcon {
		return nil, "", fmt.Errorf("unusable data: URI")
	}
	// An inline icon gets the same scrutiny as a downloaded one: it also
	// comes from someone else's page.
	if !strings.HasPrefix(http.DetectContentType(body), "image/") && !isSVG(body) {
		return nil, "", fmt.Errorf("data: URI is not an image")
	}
	return body, ext, nil
}

var contentTypeByExt = map[string]string{
	".svg":  "image/svg+xml",
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".webp": "image/webp",
	".gif":  "image/gif",
	".ico":  "image/x-icon",
}

// isSVG is separate because DetectContentType calls SVG text/xml or
// text/plain depending on how it starts.
func isSVG(body []byte) bool {
	head := strings.ToLower(string(body[:min(len(body), 512)]))
	return strings.Contains(head, "<svg")
}

// DataURI reads a stored icon and returns it as a data: URI, for the
// single-file export.
func DataURI(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return "data:" + ContentType(path) + ";base64," + base64.StdEncoding.EncodeToString(body), nil
}

// ContentType maps a stored file back to what it should be served as.
func ContentType(path string) string {
	if ct, ok := contentTypeByExt[strings.ToLower(filepath.Ext(path))]; ok {
		return ct
	}
	return "application/octet-stream"
}
