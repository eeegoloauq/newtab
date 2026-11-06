# Running on a server

## Release files

Every [release](https://github.com/eeegoloauq/newtab/releases) has binaries for linux/amd64 and
arm64, the browser extension and `SHA256SUMS`. Go is only needed to build from source.

In `config.yaml`, set `icon_dir: /var/lib/newtab`. That directory is the Docker volume and the
systemd state directory.

**Docker.** The image is 23 MB, has no shell and runs as `nobody`. Also set
`listen: 0.0.0.0:5669` in the config, put [`contrib/compose.yaml`](../contrib/compose.yaml) next to it
and run:

```sh
docker compose up -d
```

**systemd.**

```sh
install -m 0755 newtab-linux-amd64 /usr/bin/newtab
useradd --system --home /var/lib/newtab --shell /usr/sbin/nologin newtab
install -m 0644 contrib/systemd/newtab.service /etc/systemd/system/
install -d /etc/newtab && cp config.yaml /etc/newtab/
systemctl enable --now newtab
```

Put a reverse proxy in front of it, or open the port to your LAN.

[`contrib/newtab-update`](../contrib/newtab-update) updates to the latest release: it checks
`SHA256SUMS`, makes sure the new binary loads your config, and restores the old one if the service
does not respond after the restart. Its timer is in `contrib/systemd/`.

## Privacy

The page sends no referrer and loads nothing from other servers. The only state is the config and
the icon directory.
