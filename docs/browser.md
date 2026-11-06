# Start page and new tab

## Start page

- Chrome and Edge: Settings → On startup, and Appearance → Show home button.
- Firefox: Settings → Home → Homepage and new windows → Custom URLs.
- Android and iPhone: Add to Home screen from the browser menu. Chrome on Android also accepts the
  address under Settings → Homepage.

Desktop browsers cannot point the new tab page at a URL. For that, use the extension below.

## As a new tab

Chrome and Firefox only let an extension replace the new tab page, so newtab
writes one:

```sh
./newtab extension config.yaml ./ext
```

This writes a folder with the page, its icons and a manifest. Load it unpacked
(`chrome://extensions` → Developer mode → Load unpacked, or `about:debugging` in Firefox). It is a
static copy, so rebuild it when your links change. Every
[release](https://github.com/eeegoloauq/newtab/releases) includes one built from the example config.

For a live new tab backed by your running server, use `./newtab extension -url
https://newtab.example.com config.yaml ./ext` or the live extension archive in a
release. The live extension redirects each new tab to the server page.
