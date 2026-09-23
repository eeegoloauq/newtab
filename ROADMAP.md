# Roadmap

Larger work that shouldn't be done in passing. Remove an item when it ships.

- **Remove unused search prefixes.** `internal/web/page.html` and `internal/web/page.go` still emit `data-prefixes`, but `internal/web/page.js` never reads it; align the config and generated page with supported behavior.
- **Escape live extension HTML.** `cmd/newtab/main.go` inserts the configured title and redirect URL into raw HTML; escape both values and cover hostile input.
- **Cover browser behavior.** Add meaningful DOM filtering and extension tests for `internal/web/page.js` and `contrib/extension-live/`; current ranking tests do not exercise those paths.
- **Cover background handling.** Add tests for `internal/backdrop/`, which has no test file despite validating and serving user-selected images.
- **Show empty search results.** `internal/web/page.html` and `internal/web/page.js` leave an empty list when a query matches no links; add a clear no-matches state.
- **Review poller duplication.** `internal/status/`, `internal/rates/`, `internal/weather/`, and `internal/proxmox/` repeat timer and last-value handling; simplify shared behavior without coupling their clients.
