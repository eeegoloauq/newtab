# newtab

The page your browser opens instead of its own new tab. Your links on one
screen, type to filter them, Enter opens the one you meant.

![The demo page](docs/screenshot.png)

## Run it

```sh
go build ./cmd/newtab
./newtab demo                        # the page above, on made-up state

cp config.example.yaml config.yaml   # your links go here
./newtab validate config.yaml
./newtab run config.yaml
```

Icons are fetched from the sites themselves on the first run and read from
disk after that, so a page never waits on the network. A site that serves no
icon gets the globe a browser would draw; a file you put in `icon_dir` yourself
wins over the fetcher.

## Config

```yaml
sections:
  - name: Read
    style: list
    links:
      - name: Hacker News
        url: https://news.ycombinator.com/
        alias: [hn]          # also matches when you type this
```

`style: list` is bookmarks. `style: live` is for things that can be down — those
rows can show state, see below. Full example: [config.example.yaml](config.example.yaml).

## As a new tab

Chrome and Firefox only let an extension replace the new tab page, so newtab
writes one:

```sh
./newtab extension config.yaml ./ext
```

That is a folder with the page, its icons and a manifest — load it unpacked
(`chrome://extensions` → Developer mode → Load unpacked). It is a snapshot: no
server, no network, no live state. Rebuild it when the links change.

For the live version, run the server and point the extension or your homepage
setting at it.

## State from a monitor

```yaml
status:
  url: http://monitor.example/api/status
  every: 30s
  tail: exceptions
```

Rows match checks by the host they point at, or by `check:` on the link. A row
that is down says so and for how long. A healthy row shows what `tail` says:
`exceptions` (default: nothing until the last day was less than perfect),
`latency`, `uptime24h`, `uptime7d`, `quiet`.

Written for [lookout](https://github.com/eeegoloauq/lookout), but it only reads
this much, so anything can serve it:

```json
{"version": 1, "checks": [
  {"name": "Photos", "url": "https://photos.example.com/", "status": "up",
   "muted": false, "last_probe": {"duration_ms": 21},
   "uptime_24h": {"ratio": 0.9982}, "incident": null}
]}
```

The poll is on its own clock and nothing is written back, so a monitor that is
slow or down costs the page nothing.

## Numbers from Proxmox

```yaml
proxmox:
  url: https://198.51.100.10:8006
  token_env: NEWTAB_PROXMOX_TOKEN   # user@realm!id=secret, from the environment
  attach: Hypervisor                # the link whose row shows them
  insecure: true                    # it answers with its own certificate
```

That row reads `16 · 29% · 51%` — guests running, cpu, memory — and hovering it
spells that out. `PVEAuditor` is enough. The secret is not in the config file.

## Notes

The page sends no referrer, so the sites you open are not told where you came
from. Nothing on it loads from anywhere but your own server.

Binaries: [releases](https://github.com/eeegoloauq/newtab/releases).
A hardened systemd unit: [`contrib/systemd/newtab.service`](contrib/systemd/newtab.service).

MIT.
