# newtab

Start page. One Go binary, one YAML file, one HTML page. No database, no build
step, no npm, one module (a YAML parser).

## Layout

```
cmd/newtab      CLI: run, validate, render, version
internal/config YAML -> sections, and every validation error the operator gets
internal/web    the page: template, CSS, JS, and the HTTP handlers
```

## Rules

The page is rendered on the server from the config. JavaScript filters what is
already there and decides what Enter opens — it never builds markup and never
fetches. With JS off the page is still every link, which is the point.

Nothing on the page may move, fade or reflow after the first paint. This page
opens dozens of times a day; animation is a tax paid every time.

Typing does not open an overlay and does not dim the page. The list below the
field is the result set, so it stays readable while you type.

Links open in the same tab. This is the tab you opened in order to go
somewhere.

No icon is fetched from a third party. Favicons through a search engine leak
the whole link list to it; icons are served from here or not at all.

Comments say why, not what.

## Working on it

```sh
go vet ./... && go test ./...
go run ./cmd/newtab validate config.example.yaml
go run ./cmd/newtab render config.example.yaml /tmp/page.html
go run ./cmd/newtab run config.example.yaml
```

Tests must not name a real host or address — this repo will be public. Use
`example.com`, `.example`, `.invalid`, `198.51.100.0/24`.

Commit messages are a sentence saying what changed and why, in English, present
tense, no prefix tags.
