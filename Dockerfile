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

# Non-root user: UID ≥ 10001, no login shell.
RUN adduser -u 10001 -S -D -H -s /sbin/nologin appuser

COPY --from=build /mirror /usr/local/bin/mirror

# The binary must be readable and executable by the non-root user.
RUN chmod 555 /usr/local/bin/mirror

# HEALTHCHECK: verify the binary is present and executable.
# The container exits after apply/teardown, so this only runs during the brief
# window the container is alive; it serves as a basic sanity check.
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=1 \
  CMD test -x /usr/local/bin/mirror || exit 1

USER 10001

# The configuration file is expected to be bind-mounted at /config.yaml.
# Override with --config if you use a different path.
WORKDIR /
ENTRYPOINT ["mirror", "apply", "--config", "/config.yaml"]
