# container-net-mirroring

[![CI](https://github.com/YannickLand/container-net-mirroring/actions/workflows/ci.yml/badge.svg)](https://github.com/YannickLand/container-net-mirroring/actions/workflows/ci.yml)
[![Coverage](https://codecov.io/gh/YannickLand/container-net-mirroring/branch/main/graph/badge.svg)](https://codecov.io/gh/YannickLand/container-net-mirroring)
[![Go 1.22](https://img.shields.io/badge/go-1.22-blue.svg)](https://golang.org/)
[![DOI](https://zenodo.org/badge/DOI/10.5281/zenodo.18925417.svg)](https://doi.org/10.5281/zenodo.18925417)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**Read-only network access for Docker containers via configurable traffic mirroring.**

This tool enables a container to observe host-level network traffic without the
unsafe `network_mode: host` setting. It configures Linux Traffic Control (tc)
mirroring rules on the host that redirect matching packets into the container's
virtual ethernet interface — giving the container a read-only view of precisely
the traffic it needs, and nothing more.

## Motivation

The standard approach to giving a container access to host network traffic is
`network_mode: host`, which removes all network namespace isolation and grants
unrestricted read/write access to every host interface. Traffic mirroring
achieves the same observability goal at a fraction of the attack surface:

| | `network_mode: host` | Traffic mirroring |
|---|---|---|
| Container can send packets | ✓ | ✗ |
| Exposes all host interfaces | ✓ | ✗ |
| Scope is configurable | ✗ | ✓ |
| Requires `NET_ADMIN` on | container | setup container only |

The setup container runs once, installs the tc rules, and exits. The observer
container itself requires no elevated capabilities.

## How it works

```
┌─────── Host ─────────────────────────────────────────────────┐
│                                                               │
│  eth0 ──► [ingress qdisc] ──► [u32 filter] ─── mirror ──►  │
│                                                    │          │
│                              ┌─────────────────────┘          │
│                              ▼                                │
│                           veth0  ◄── container network ns    │
│                              │                                │
│                       ┌──────┴──────┐                        │
│                       │ net-observer│  (tcpdump, tshark, …)  │
│                       └─────────────┘                        │
└───────────────────────────────────────────────────────────────┘
```

1. `mirror apply` reads `config.yaml` and validates all entries.
2. For each mirroring rule it resolves the host interface name(s) and the
   container's host-side veth interface via the Docker API.
3. It installs a tc qdisc (ingress or prio/root) and a U32 filter with a
   `mirred egress mirror` action pointing at the container veth.
4. `mirror teardown` removes the qdiscs (and their filters) when done.

## Quickstart

### 1. Copy the example configuration

```bash
cp config.example.yaml config.yaml
```

Edit `config.yaml` to describe which traffic to mirror and which container to
mirror it into. See [Configuration](#configuration) for the full reference.

### 2. Run via Docker Compose

```yaml
# docker-compose.yml (excerpt)
services:
  net-observer:
    container_name: net-observer
    image: nicolaka/netshoot
    command: ["tcpdump", "-i", "any", "-n"]

  mirror-setup:
    image: ghcr.io/yannickland/container-net-mirroring:latest
    depends_on:
      net-observer:
        condition: service_started
    network_mode: host
    cap_add: [NET_ADMIN, NET_RAW]
    volumes:
      - /var/run:/var/run
      - /sys/class/net:/sys/class/net:ro
      - ./config.yaml:/config.yaml:ro
    restart: "no"
```

See [`docker-compose.example.yml`](docker-compose.example.yml) for a complete
annotated example.

### 3. Or run directly

```bash
# Build
docker build -t mirror-setup .

# Start your observer container first
docker run -d --name net-observer nicolaka/netshoot tcpdump -i any -n

# Apply mirroring
docker run --rm \
  --network=host \
  --cap-add=NET_ADMIN --cap-add=NET_RAW \
  -v /var/run:/var/run \
  -v /sys/class/net:/sys/class/net:ro \
  -v $(pwd)/config.yaml:/config.yaml:ro \
  mirror-setup

# Teardown when done
docker run --rm \
  --network=host \
  --cap-add=NET_ADMIN \
  -v /sys/class/net:/sys/class/net:ro \
  -v $(pwd)/config.yaml:/config.yaml:ro \
  mirror-setup teardown --config /config.yaml
```

## Configuration

`config.yaml` contains a list of mirroring rules under the `mirrorings` key.

```yaml
mirrorings:
  - interface: <interface>
    direction: ingress | egress | both
    container: <container-name>
    traffic: <preset> | <custom-filter>
```

### `interface`

| Value | Resolves to |
|---|---|
| `default` | The interface used by the default IPv4 route (e.g. `eth0`) |
| `loopback` | The loopback interface (`lo`) |
| `all-physical` | Every non-virtual interface on the host |
| `<name>` | A literal interface name (e.g. `ens3`, `bond0`) |

### `direction`

`ingress` captures packets arriving on the interface; `egress` captures packets
leaving; `both` does both (generates two tc rules).

### `traffic` — preset names

Using a named preset is the recommended approach. Presets embody the principle
of least privilege: choose the most specific preset that satisfies your use case.

| Preset | Mirrors |
|---|---|
| `icmp` | ICMP (ping, traceroute) |
| `dns` | UDP + TCP port 53 |
| `ntp` | UDP port 123 |
| `web` | TCP ports 80 and 443 |
| `ssh` | TCP port 22 |
| `all` | Everything ⚠ emits a least-privilege warning |

### `traffic` — custom filter

```yaml
traffic:
  protocol: tcp | udp | icmp | all   # required
  port: <1–65535>                    # destination port (tcp/udp only)
  src: <IP or CIDR>                  # e.g. 10.0.0.0/8
  dst: <IP or CIDR>
```

All fields except `protocol` are optional and can be freely combined.

### Full example

```yaml
mirrorings:

  # Least-privilege: mirror only ICMP on the default interface
  - interface: default
    direction: ingress
    container: net-observer
    traffic: icmp

  # Mirror DNS in both directions
  - interface: default
    direction: both
    container: net-observer
    traffic: dns

  # Custom: TCP:8080 from 10.0.0.0/8 only
  - interface: default
    direction: ingress
    container: net-observer
    traffic:
      protocol: tcp
      port: 8080
      src: 10.0.0.0/8
```

## Documentation

| Type | Document |
|---|---|
| Tutorial | [Getting Started: your first traffic mirror](docs/tutorials/getting-started.md) |
| How-to | [Configure a custom traffic filter](docs/how-to-guides/how-to-custom-filter.md) |
| How-to | [Mirror multiple interfaces](docs/how-to-guides/how-to-multiple-interfaces.md) |
| How-to | [Tear down mirroring rules](docs/how-to-guides/how-to-teardown.md) |
| Explanation | [Architecture and design](docs/explanation/architecture.md) |
| Explanation | [Security model](docs/explanation/security-model.md) |
| Reference | [CLI reference](docs/reference/cli.md) |
| Reference | [Configuration file reference](docs/reference/configuration.md) |

## CLI reference

```
mirror apply    [--config <file>]   Apply mirroring rules (default: config.yaml)
mirror teardown [--config <file>]   Remove mirroring rules
mirror --help                       Show help
```

## Requirements

The setup container needs:

| Requirement | Why |
|---|---|
| `NET_ADMIN` capability | To configure tc qdiscs and filters |
| `NET_RAW` capability | To install packet-inspection hooks |
| `network_mode: host` | To address host-side network interfaces |
| `/var/run` volume | Docker socket access for veth resolution |
| `/sys/class/net` volume | Interface metadata (read-only sufficient) |

The **observer container** needs none of these.

## Building from source

The tool targets Linux exclusively — it relies on the Linux Traffic Control
subsystem (tc/netlink) and Linux procfs, which are not available on Windows or
macOS. You can build from any OS using Go's cross-compilation support; the
resulting binary must run on a Linux host (or inside a Linux container).

```bash
# Cross-compile from any OS for Linux/amd64
GOOS=linux GOARCH=amd64 go build -o mirror ./cmd/mirror
```

Requires Go 1.22 or later. The resulting binary is statically linked and has
no runtime dependencies beyond a Linux kernel with tc support (≥ 3.18).

## Development

```bash
# Run unit tests (platform-independent config package)
go test ./internal/config/...

# Run all tests (requires Linux)
go test ./...

# Lint
golangci-lint run

# Security scan — dependencies
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...

# Build the binary (Linux target)
GOOS=linux GOARCH=amd64 go build -o mirror ./cmd/mirror

# Build the Docker image
docker build -t mirror-setup .
```

Integration tests (iface, tc, docker packages) require a Linux host with
`NET_ADMIN` and a running Docker daemon. See [tests/TODO.md](tests/TODO.md) for
details.

## Security considerations

- The mirrored traffic is a **read-only copy** — the observer container cannot
  inject or modify packets on the host.
- Use the most specific preset or filter to minimise exposure.
- The `all` preset is provided for diagnostics only; it should not be used in
  production deployments.
- After the observer container is stopped, run `mirror teardown` to remove the
  tc rules and restore the default interface configuration.

## License

[MIT](LICENSE)
