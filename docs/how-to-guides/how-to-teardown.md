# How to Tear Down Mirroring Rules

## When to use this

Run `mirror teardown` after your observer container has finished its work —
or before a host reboot to ensure tc state is clean. Teardown is idempotent:
running it when no rules are active is safe and returns exit code 0.

## Linux / macOS

```bash
docker run --rm \
  --network=host \
  --cap-add=NET_ADMIN \
  -v /sys/class/net:/sys/class/net:ro \
  -v $(pwd)/config.yaml:/config.yaml:ro \
  ghcr.io/yannickland/container-net-mirroring:latest teardown --config /config.yaml
```

## PowerShell

```powershell
docker run --rm `
  --network=host `
  --cap-add=NET_ADMIN `
  -v /sys/class/net:/sys/class/net:ro `
  -v "${PWD}/config.yaml:/config.yaml:ro" `
  ghcr.io/yannickland/container-net-mirroring:latest teardown --config /config.yaml
```

## Docker Compose

If you used Docker Compose to apply rules, run teardown as an additional
one-shot service:

```yaml
services:
  mirror-teardown:
    image: ghcr.io/yannickland/container-net-mirroring:latest
    command: ["teardown", "--config", "/config.yaml"]
    network_mode: host
    cap_add: [NET_ADMIN]
    volumes:
      - /sys/class/net:/sys/class/net:ro
      - ./config.yaml:/config.yaml:ro
    restart: "no"
```

```bash
docker compose run --rm mirror-teardown
```

## Verifying removal

After teardown, confirm no mirroring qdiscs remain on the interface:

```bash
tc qdisc show dev eth0
```

The output should no longer contain an `ingress` qdisc or a `prio` qdisc with
a mirred filter. Running `tc filter show dev eth0 parent ffff:` should return
no output.

## See also

- [CLI reference](../reference/cli.md)
- [Getting Started tutorial](../tutorials/getting-started.md)
