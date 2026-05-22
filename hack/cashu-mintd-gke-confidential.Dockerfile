# syntax=docker/dockerfile:1.7

ARG MINTD_BASE_IMAGE=cashubtc/mintd@sha256:1551b1b56f8670942164d3831ad00b54d662310bc811458b413a59ffcc7a152e

FROM --platform=$BUILDPLATFORM golang:1.25 AS builder
WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY api/ api/
COPY cmd/cashu-attested-entrypoint/ cmd/cashu-attested-entrypoint/
COPY internal/attestedentrypoint/ internal/attestedentrypoint/

ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/cashu-attested-entrypoint ./cmd/cashu-attested-entrypoint

FROM ${MINTD_BASE_IMAGE}
COPY --from=builder /out/cashu-attested-entrypoint /usr/local/bin/cashu-attested-entrypoint
ENTRYPOINT ["/usr/local/bin/cashu-attested-entrypoint"]
