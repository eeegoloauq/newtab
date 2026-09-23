# newtab

A configurable browser start page for people who keep their links in YAML. A Go server renders the page and optional status, weather, rate, and hypervisor data; it can also generate a standalone page or browser extension.

## Working commands

```sh
go build ./cmd/newtab
go test ./...
go test -race ./...
go vet ./...
go run ./cmd/newtab demo
go run ./cmd/newtab validate config.example.yaml
go run ./cmd/newtab render config.example.yaml /tmp/page.html
go run ./cmd/newtab run config.example.yaml
```

The CLI also provides `icons`, `extension`, and `version`; see `go run ./cmd/newtab` for usage. The ranking test skips if Node is unavailable.

## Decisions and gotchas

- Keep the page usable with JavaScript disabled. Filtering acts on server-rendered links; it does not fetch data while the page is open. Keep links in the same tab and avoid animation or layout shifts. [Previous design decisions](docs/design.md) explain why tiles, CSS columns, on-demand icon proxying, and editing links in the page were rejected.
- Missing icons are fetched in the background when the server starts, or explicitly with `newtab icons`; page requests do not wait for them. The fetcher uses a browser user agent and disables TLS verification for sites with self-signed certificates. Stored icons are untrusted: retain `nosniff` and the sandbox CSP when serving them.
- Integration secrets come from environment variables. Validation fails if a configured variable is empty. Pollers retain their last good response; rendering never waits on a monitor.
- Keep configuration as the source of truth. Site copy belongs in the config's `text:` values, with defaults supplied when fields are added. Do not add a database or an in-page config editor.
- Use example domains and reserved documentation addresses in tests and docs. This repository may be public.

## Release and recovery

A `v*` tag or manual GitHub release workflow builds Linux amd64/arm64 binaries, extension archives, checksums, and container images. Deploy the image with `contrib/compose.yaml` or the binary with `contrib/systemd/newtab.service`. The optional `contrib/newtab-update` verifies the checksum and running config before replacement, then checks `/healthz` and restores the previous binary if the restart fails. Keep that validation and rollback order.

Planned larger work: `ROADMAP.md`.
