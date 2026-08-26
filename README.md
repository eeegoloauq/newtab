# newtab

A start page for the browser: every link on one screen, filtered as you type.

![The demo page](docs/screenshot.png)

One Go binary, one YAML file, one HTML page. No build step, no npm, no
framework, and nothing on the page talks to anyone but this server.

- **One screen.** No tabs, no folders, no hunting for a group.
- **Type anywhere.** The first keystroke lands in the field and the links filter
  in place; Enter opens the first match, or searches the web with what you
  typed. Arrows walk the matches, Escape clears.
- **Icons from the sites themselves,** fetched once and served from here. Never
  through a favicon service — asking one for your links hands it the list.
- **Optional state.** A row can show what an uptime monitor knows about it, and
  a hypervisor can put its numbers on its own row.

## Try it

```sh
go build ./cmd/newtab
./newtab demo            # the page above, on invented state
```

## Use it

```sh
cp config.example.yaml config.yaml   # edit it
./newtab validate config.yaml
./newtab icons config.yaml           # fetch each site's own icon
./newtab run config.yaml
```

A section is either `live` — things that can be down, whose rows carry state —
or `list`, plain bookmarks. Both are lists in the same columns.

```yaml
sections:
  - name: Read
    style: list
    links:
      - name: Hacker News
        url: https://news.ycombinator.com/
        alias: [hn]          # also matches when you type this
```

The page shows the browser's globe for a site that serves no icon of its own,
and a file dropped into `icon_dir` by hand always beats the fetcher.

`newtab render -inline config.yaml page.html` writes the whole page, icons
embedded, as a single file — useful inside a browser extension, or for when the
server is not reachable.

## State from a monitor

```yaml
status:
  url: http://monitor.example/api/status
  every: 30s
  tail: exceptions
```

Rows are matched to checks by the host they point at, or by `check:` on the
link. A row that is down says so and for how long. What a healthy row says is
`tail`: `exceptions` (the default — nothing, until the last day was less than
perfect), `latency`, `uptime24h`, `uptime7d` or `quiet`.

Written against [lookout](https://github.com/eeegoloauq/lookout), but the
document it reads is small enough to serve from anything:

```json
{"version": 1, "checks": [
  {"name": "Photos", "url": "https://photos.example.com/", "status": "up",
   "muted": false, "last_probe": {"duration_ms": 21},
   "uptime_24h": {"ratio": 0.9982}, "incident": null}
]}
```

Nothing is ever written back, the poll runs on its own clock, and a monitor
that is unreachable leaves the rows quiet rather than blank.

## Numbers from Proxmox

```yaml
proxmox:
  url: https://198.51.100.10:8006
  token_env: NEWTAB_PROXMOX_TOKEN   # user@realm!id=secret, from the environment
  attach: Hypervisor                # the link whose row shows them
  insecure: true                    # it answers with its own certificate
```

The row reads `16 · 29% · 51%` — guests running, cpu, memory — and the tooltip
spells that out. A token holding `PVEAuditor` is enough; the secret stays out
of the config file.

## Install

Binaries for linux/amd64 and arm64 are attached to each
[release](https://github.com/eeegoloauq/newtab/releases). It wants no state
beyond its config and its icon directory; `contrib/systemd/newtab.service` is a
hardened unit to copy.

## License

MIT
