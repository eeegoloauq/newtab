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

Sections come in two styles: `cards` for things that can be down (they get a
tile and, later, a status dot), `list` for plain bookmarks. See
[config.example.yaml](config.example.yaml).

## Status

Early. Statuses from [lookout](https://github.com/eeegoloauq/lookout) and cached
icons are next.

## License

MIT
