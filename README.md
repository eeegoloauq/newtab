# newtab

A start page for the browser: one screen, every link on it, filtered as you type.

- **One page.** No tabs, no folders, no scrolling to find a group.
- **Type anywhere.** The first keystroke lands in the field; the links below
  filter in place. Enter opens the first match, or searches the web with what
  you typed. Escape clears.
- **Server-rendered.** Go's `html/template`, CSS and 60 lines of JS, all inline
  in the response. No build step, no bundler, no framework.
- **Nothing phones home.** No CDN, no analytics, no favicon service.

```sh
go build ./cmd/newtab
./newtab validate config.example.yaml
./newtab run config.example.yaml
```

Sections come in two styles: `live` for things that can be down — their rows
carry a status and a tail for a number — and `list` for plain bookmarks. Both
are lists in the same columns. See [config.example.yaml](config.example.yaml).

Icons are fetched once, from each site itself:

```sh
./newtab icons config.example.yaml     # writes into icon_dir
./newtab render -inline config.yaml page.html   # one file, icons embedded
```

The page shows the browser's globe for a site that serves no icon, and a file
dropped into `icon_dir` by hand always wins over the fetcher.

## State from a monitor

A `live` section can show what an uptime monitor knows. Point it at a
[lookout](https://github.com/eeegoloauq/lookout) instance:

```yaml
status:
  url: http://monitor.example/api/status
  every: 30s
```

Rows are matched to checks by the host they point at, or by an explicit
`check:` on the link. A matched row shows its last latency, or how long it has
been down. Nothing is ever written back, the poll runs on its own clock, and a
monitor that is unreachable simply leaves the rows quiet.

## Status

Early, and in daily use by its author.

## License

MIT
