# How to Mirror Multiple Interfaces

## When to use this

Use multiple mirroring rules when your host has more than one relevant network
interface — for example, a management interface (`eth0`) and a data-plane
interface (`eth1`) — and you want the observer container to see traffic from
both.

The `all-physical` logical name is the simplest option when you want to capture
traffic from every non-virtual interface at once.

## Using `all-physical`

```yaml
mirrorings:
  - interface: all-physical
    direction: ingress
    container: net-observer
    traffic: web
```

`all-physical` resolves at apply time to every interface that is not under
`/sys/devices/virtual/net/`. The tool installs one tc rule per resolved
interface automatically.

## Listing rules for distinct interfaces

When different interfaces need different filters, add one entry per interface:

```yaml
mirrorings:
  - interface: eth0
    direction: ingress
    container: net-observer
    traffic: dns

  - interface: eth1
    direction: both
    container: net-observer
    traffic: web
```

Each entry is independent: filters, directions, and traffic presets can differ.

## Tearing down

`mirror teardown` reads the same config file and removes the qdiscs from every
interface listed (or resolved by `all-physical`). Run it with the same
`config.yaml` that was used for `apply`:

```bash
docker run --rm \
  --network=host --cap-add=NET_ADMIN \
  -v /sys/class/net:/sys/class/net:ro \
  -v $(pwd)/config.yaml:/config.yaml:ro \
  ghcr.io/yannickland/container-net-mirroring:latest teardown --config /config.yaml
```

## See also

- [Configuration file reference — interface field](../reference/configuration.md#interface)
- [How to tear down mirroring rules](how-to-teardown.md)
