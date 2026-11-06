# Development

Go (version in `go.mod`). The checks CI runs:

```sh
go vet ./...
go test -race ./...
go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...
CGO_ENABLED=0 go build ./cmd/newtab
go run ./cmd/newtab validate config.example.yaml
go run ./cmd/newtab render config.example.yaml /tmp/page.html
```

`go run ./cmd/newtab demo` serves the page with made-up data. The ranking test skips if Node is
not installed.

A `v*` tag builds the release: Linux binaries, both extension archives, `SHA256SUMS` and the
`ghcr.io/eeegoloauq/newtab` image. The tag message is the release headline;
[`contrib/release-install.md`](../contrib/release-install.md) is appended to it.

[Rejected designs](design.md) · [Roadmap](../ROADMAP.md)
