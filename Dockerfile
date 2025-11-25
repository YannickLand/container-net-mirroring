# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Cross-compile a static binary targeting Linux/amd64.
# CGO_ENABLED=0 produces a fully static binary suitable for a scratch image.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /mirror ./cmd/mirror

# ── Runtime stage ─────────────────────────────────────────────────────────────
# Use a minimal base that still provides Docker CLI and iproute2 for runtime
# operations. Alpine is significantly smaller than debian-based images.
FROM alpine:3.20

RUN apk add --no-cache \
    iproute2=6.9.0-r0 \
    docker-cli=26.1.5-r0

COPY --from=build /mirror /usr/local/bin/mirror

# The configuration file is expected to be bind-mounted at /config.yaml.
# Override with --config if you use a different path.
WORKDIR /
ENTRYPOINT ["mirror", "apply", "--config", "/config.yaml"]
