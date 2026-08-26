# Two stages, and the second one is empty: the binary is static and the
# only other thing it needs is the certificate bundle, for reading a
# monitor or a site's icon over https.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /newtab ./cmd/newtab

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /newtab /newtab
# Nothing here needs root, and there is no shell to become anything else.
USER 65534:65534
# The config is read-only; the icon directory is the only thing written,
# and only by `newtab icons`.
VOLUME /var/lib/newtab
EXPOSE 5669
ENTRYPOINT ["/newtab"]
CMD ["run", "/etc/newtab/config.yaml"]
