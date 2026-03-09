# Architecture and Design

## Overview

`container-net-mirroring` configures Linux Traffic Control (tc) mirroring rules
on the host that redirect a copy of matching packets into a container's virtual
ethernet (veth) interface. The container receives a read-only view of the
selected traffic without any elevated capabilities of its own.

## Why tc mirroring instead of `network_mode: host`

`network_mode: host` removes all network namespace isolation and gives the
container unrestricted read/write access to every host interface. Traffic
mirroring achieves the same observability goal at a fraction of the attack
surface:

| | `network_mode: host` | Traffic mirroring |
|---|---|---|
| Container can send packets | ✓ | ✗ |
| Exposes all host interfaces | ✓ | ✗ |
| Scope is configurable | ✗ | ✓ |
| Where NET_ADMIN is needed | container | setup container only |

The setup container runs once to install the tc rules, then exits. The observer
container itself requires no elevated capabilities.

## Data flow

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

1. `mirror apply` reads `config.yaml` and validates every entry.
2. For each rule it resolves the host interface name(s) and the container's
   host-side veth via the Docker API.
3. It installs a tc qdisc (ingress or prio/root) and a U32 filter with a
   `mirred egress mirror` action targeting the container veth.
4. `mirror teardown` removes the qdiscs (and their attached filters).

## Key Linux tc concepts

### Qdiscs

A **qdisc** (queuing discipline) is a kernel object attached to a network
interface that controls how packets are processed. This tool uses two qdisc
types:

- **ingress qdisc** (`handle ffff:`) — receives incoming packets before they
  are delivered to the host network stack.
- **prio qdisc** (`handle 1:0`) — attached at the root for egress; it
  immediately dequeues packets so egress filters can run.

### U32 filters

A **U32 filter** matches against arbitrary 32-bit words in a packet header at
a configurable offset. This tool builds U32 selectors for IP protocol number,
destination port, source/destination CIDR, and Ethernet protocol. See
`internal/tc/tc.go` for the byte-order details (`swap32` and the IPv4 header
offset assumptions).

### Mirred action

The **mirred** (mirror and redirect) action sends a copy of a matched packet to
a different interface without consuming the original. The `TCA_EGRESS_MIRROR`
flag ensures the original packet continues on its normal path.

## veth resolution

Each Docker container that is not in `network_mode: host` has a virtual
ethernet pair: one end inside the container namespace (`eth0`) and one end on
the host (typically `vethXXXXXX`). The tool resolves the host-side interface
name by:

1. Calling the Docker API `ContainerInspect` to get the container's PID.
2. Reading `/proc/<pid>/root/sys/class/net/eth0/iflink` — accessible from the
   host without entering the network namespace.
3. Resolving the link index to an interface name via Netlink.

## Package structure

```
cmd/mirror/       CLI entry point (apply / teardown commands)
internal/
  config/         YAML loading, validation, preset registry
  docker/         veth resolution via Docker API
  iface/          logical → real interface name resolution
  tc/             netlink-based qdisc and filter management
```

## Design decisions

- **Static binary**: `CGO_ENABLED=0` produces a fully static binary suitable
  for a minimal Alpine runtime image.
- **Idempotent apply**: Existing qdiscs are reused; only missing filters are
  added. Applying the same config twice is safe.
- **Idempotent teardown**: Missing qdiscs are silently skipped; teardown of an
  already-clean interface returns success.
- **Preset registry**: Named presets encode least-privilege intent. The `all`
  preset emits an over-privilege warning to discourage production use.
