# Security Model

## Principle of least privilege

The core design goal is to give container workloads the minimum network
visibility needed, using the minimum host privileges needed.

### Observer container

The **observer container** (e.g. running `tcpdump`) requires **no elevated
capabilities** and no special network mode. It only needs to read packets that
arrive on its own network interface. Those packets are delivered by the kernel
as mirrored copies, so the container cannot:

- Send packets on the host interfaces.
- See traffic that is not selected by the mirroring filter.
- Modify packets in transit.

### Setup container

The **setup container** runs once, installs the tc rules, and exits. It
requires:

| Requirement | Why |
|---|---|
| `NET_ADMIN` | To configure tc qdiscs and filters via Netlink |
| `NET_RAW` | To install packet-inspection hooks |
| `network_mode: host` | To address host-side network interfaces |
| `/var/run` volume | Docker socket access for veth resolution |
| `/sys/class/net` volume | Interface metadata (read-only) |

Because the setup container exits immediately after applying rules, the window
during which elevated capabilities are held is minimal.

## Traffic scope

Use the most specific preset or custom filter that satisfies your use case:

- Prefer `icmp`, `dns`, `web`, `ssh`, or `ntp` over `all`.
- Use custom filters to restrict by source/destination CIDR or exact port.
- The `all` preset emits an over-privilege warning at apply time as a reminder
  to narrow scope before production use.

## tc rules and the host network

The mirroring rules installed by this tool are purely **additive**: they add a
qdisc and a filter, but do not alter routing, firewall rules, or interface
addresses. Teardown removes the qdisc (and all attached filters) and restores
the original interface state.

Running the tool on an already-configured host is safe: existing qdiscs are
reused and only missing filters are added.

## Docker socket access

The Docker socket (`/var/run/docker.sock`) is mounted into the setup container
to resolve the container's veth interface index. The tool makes two API calls:
`ContainerInspect` (read the PID) and reads `/proc/<pid>/root/sys/class/net`
(read iflink). No containers are started, stopped, or modified.

## See also

- [Architecture and design](architecture.md) — how tc mirroring works
- [Configuration file reference](../reference/configuration.md) — traffic
  preset definitions
