# newtab

A self-hosted browser start page for Linux servers and Docker: your links on one screen, filtered
as you type.

[![Release](https://img.shields.io/github/v/release/eeegoloauq/newtab?label=release)](https://github.com/eeegoloauq/newtab/releases/latest)
[![CI](https://github.com/eeegoloauq/newtab/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/eeegoloauq/newtab/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

![The demo page](docs/screenshot.png)

- Links from one YAML file
- Service status from an uptime monitor
- Weather and exchange rates, no accounts
- Guests, CPU and memory from Proxmox
- New tab extension for Chrome and Firefox
- Loads nothing from other servers

## Download

Get the [latest release](https://github.com/eeegoloauq/newtab/releases/latest).

## Usage

```sh
./newtab demo                        # the page above, with made-up data
cp config.example.yaml config.yaml   # your links go here
./newtab run config.yaml
```

[Configuration](docs/configuration.md) · [Server setup](docs/server.md) · [Start page and new tab](docs/browser.md)

## Development

[Build and test](docs/development.md) · [Rejected designs](docs/design.md) · [Roadmap](ROADMAP.md)

## License

[MIT](LICENSE)
