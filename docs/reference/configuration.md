# Configuration File Reference

The configuration file (default: `config.yaml`) contains a single top-level
key `mirrorings` with a list of mirroring rules.

## Top-level structure

```yaml
mirrorings:
  - <mirroring rule>
  - <mirroring rule>
```

At least one rule is required.

---

## Mirroring rule fields

### `interface`

**Type:** string  
**Required:** yes

The host-side network interface to mirror traffic from.

| Value | Resolves to |
|---|---|
| `default` | The interface used by the default IPv4 route (e.g. `eth0`) |
| `loopback` | The loopback interface (`lo`) |
| `all-physical` | Every non-virtual interface on the host |
| `<name>` | A literal interface name (e.g. `ens3`, `bond0`) |

Resolution happens at `apply` time, not at parse time. Specifying a literal
name that does not exist causes `apply` to fail with an informative error.

---

### `direction`

**Type:** string  
**Required:** yes  
**Accepted values:** `ingress`, `egress`, `both`

| Value | Effect |
|---|---|
| `ingress` | Mirrors packets arriving on the interface |
| `egress` | Mirrors packets leaving the interface |
| `both` | Installs two tc rules — one for each direction |

---

### `container`

**Type:** string  
**Required:** yes

The name of the target Docker container, exactly as it appears in `docker ps`.
The container must be running when `mirror apply` is executed.

---

### `traffic` — preset names

**Type:** string (one of the preset names below)

Using a named preset is recommended. Presets encode the principle of least
privilege — choose the most specific preset that covers your use case.

| Preset | Mirrors | Notes |
|---|---|---|
| `icmp` | ICMP (ping, traceroute) | |
| `dns` | UDP + TCP port 53 | 2 rules |
| `ntp` | UDP port 123 | |
| `web` | TCP ports 80 and 443 | 2 rules |
| `ssh` | TCP port 22 | |
| `all` | All traffic | ⚠ emits an over-privilege warning |

---

### `traffic` — custom filter

**Type:** mapping

Use a custom filter when no preset matches your use case.

```yaml
traffic:
  protocol: tcp | udp | icmp | all   # required
  port: <1–65535>                    # destination port (tcp/udp only)
  src: <IP or CIDR>                  # e.g. 10.0.0.0/8 or 192.168.1.1
  dst: <IP or CIDR>
```

| Field | Type | Required | Description |
|---|---|---|---|
| `protocol` | string | yes | `tcp`, `udp`, `icmp`, or `all` |
| `port` | integer | no | Destination port, 1–65535. Valid for `tcp` and `udp` only. |
| `src` | string | no | Source IP address or CIDR block |
| `dst` | string | no | Destination IP address or CIDR block |

Validation errors:
- `port` with `protocol: icmp` or `protocol: all` → error
- `port` outside 1–65535 → error
- `src` or `dst` that is not a valid IP address or CIDR → error

---

## Validation

`mirror apply` validates the entire configuration before making any changes. If
any rule is invalid the tool exits with a non-zero code and prints the
offending field path (e.g. `mirrorings[2]: invalid direction "sideways"`).

## Full example

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

  # Mirror all physical interfaces (resolved at apply time)
  - interface: all-physical
    direction: ingress
    container: net-observer
    traffic: web
```

## See also

- [CLI reference](cli.md)
- [How to configure a custom traffic filter](../how-to-guides/how-to-custom-filter.md)
- [Security model](../explanation/security-model.md)
