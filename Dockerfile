# Two stages, and the second one is empty: the binary is static and the
# only other thing it needs is the certificate bundle, for reading a
# monitor or a site's icon over https.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /newtab ./cmd/newtab
# scratch has no mkdir: the icon directory is made here, owned by the
# runtime user, so the volume starts out writable for it.
RUN install -d -o 65534 -g 65534 /out/var/lib/newtab

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /newtab /newtab
COPY --from=build --chown=65534:65534 /out/var/lib/newtab /var/lib/newtab
# Nothing here needs root, and there is no shell to become anything else.
USER 65534:65534
# The config is read-only; site icons are written to the volume.
VOLUME /var/lib/newtab
EXPOSE 5669
ENTRYPOINT ["/newtab"]
CMD ["run", "/etc/newtab/config.yaml"]
