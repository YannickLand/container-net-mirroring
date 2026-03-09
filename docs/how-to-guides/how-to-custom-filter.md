# How to Configure a Custom Traffic Filter

## When to use this

Use a custom filter when none of the built-in presets (`icmp`, `dns`, `web`,
`ssh`, `ntp`, `all`) match your use case — for example, mirroring a proprietary
application protocol on a specific port, or restricting mirroring to traffic
from a particular subnet.

For common protocols, prefer a [preset](../reference/configuration.md#traffic--preset-names)
over a custom filter: presets are reviewed for least-privilege and self-document
intent.

## Custom filter structure

Replace the `traffic` value with a mapping:

```yaml
mirrorings:
  - interface: default
    direction: ingress
    container: net-observer
    traffic:
      protocol: tcp        # required: tcp | udp | icmp | all
      port: 8080           # optional: destination port (tcp/udp only)
      src: 10.0.0.0/8      # optional: source IP or CIDR
      dst: 192.168.1.50    # optional: destination IP or CIDR
```

All fields except `protocol` are optional and can be combined freely.

## Examples

### Mirror a single application port

```yaml
traffic:
  protocol: tcp
  port: 9000
```

### Mirror traffic from a specific subnet only

```yaml
traffic:
  protocol: tcp
  port: 443
  src: 10.0.0.0/8
```

### Mirror all UDP traffic

```yaml
traffic:
  protocol: udp
```

### Mirror all traffic (diagnostic use only)

Use the `all` preset instead of `protocol: all` — the preset emits an
over-privilege warning to remind you to narrow the scope when done:

```yaml
traffic: all
```

## Validating your configuration

Run `mirror --help` or attempt to apply the configuration with a dry run by
inspecting the output before traffic starts flowing:

```bash
docker run --rm \
  --network=host --cap-add=NET_ADMIN --cap-add=NET_RAW \
  -v /var/run:/var/run \
  -v /sys/class/net:/sys/class/net:ro \
  -v $(pwd)/config.yaml:/config.yaml:ro \
  ghcr.io/yannickland/container-net-mirroring:latest
```

If validation fails, `mirror` exits with a non-zero code and prints the
offending field.

## See also

- [Configuration file reference](../reference/configuration.md) — all fields
  and allowed values.
- [How to mirror multiple interfaces](how-to-multiple-interfaces.md) — applying
  the same filter across several host interfaces.
