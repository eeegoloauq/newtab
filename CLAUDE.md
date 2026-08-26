# newtab

Start page. Renders one HTML page from a YAML file, fetches the icon of every
site it links to, and reads a monitor and a hypervisor to put their numbers on
the matching rows.

## Layout

```
cmd/newtab      CLI: run, validate, render, icons, version
internal/config YAML -> sections, and every validation error the operator gets
internal/icons  fetching a site's own favicon, and the store it lands in
internal/status  read-only client for a lookout monitor, polled on its own clock
internal/weather read-only client for Open-Meteo, polled on its own clock
internal/proxmox read-only client for a hypervisor's three numbers
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

Icons are fetched ahead of time, never while a page is being served. A server
that renders by proxying an icon on demand would make the first paint wait on
forty other people's websites, and would hand every one of them the moment you
open your browser. `newtab icons` walks to every URL in the config, with a browser's user agent
and with TLS verification off. Both are deliberate: sites behind a bot filter
answer 403 to anything less, and half the boxes on a home LAN serve a
self-signed certificate. It follows that stored icons are untrusted bytes, so
`/icon/` serves them under `nosniff` and a sandbox CSP. Do not "fix" any of
those three by removing them.

Secrets are read from the environment, never from the config: the config is in
version control and the secret is not. `newtab validate` fails when the named
variable is empty, because a page that silently drops an integration is worse
than one that will not start.

A request never waits on the monitor. The poller keeps the last answer in
memory and a failed poll keeps it: a monitor restart is not an outage of
everything. Nothing is ever written back to the monitor — it owns what is up,
this is a reader.

The columns are dealt on the server. CSS multicol was tried and it rebalanced
on every keystroke, throwing sections sideways while the filter ran.

Anything the page reads from elsewhere is polled on its own clock by a package
of its own, and handed to the renderer as a value. There is no fetching inside
a render and no shared mutable state between them.

The theme is four colours and two numbers, and every one of them is validated
before it reaches the stylesheet. Do not widen it into a styling language: the
page is an index and it has to stay readable.

The page ships no natural language. Strings that are not a link name live in
`text:` in the config, with English defaults in `applyDefaults` — add the
default in the same change as the field.

Comments say why, not what.

## Working on it

```sh
go vet ./... && go test ./...
go run ./cmd/newtab validate config.example.yaml
go run ./cmd/newtab render config.example.yaml /tmp/page.html
go run ./cmd/newtab run config.example.yaml
```

Tests must not name a real host or address — this repo is public. Use
`example.com`, `.example`, `.invalid`, `198.51.100.0/24`. Non-ASCII names in
tests are data, not prose: they cover the slug path and should stay.

What has been tried and thrown away is in [docs/design.md](docs/design.md).
Read it before reintroducing tiles, monograms or a database.

Commit messages are a sentence saying what changed and why, in English, present
tense, no prefix tags.
