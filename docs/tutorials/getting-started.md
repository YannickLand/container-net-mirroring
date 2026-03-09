# Getting Started: your first traffic mirror

This tutorial walks you through installing the tc mirroring rules so that a
container called `net-observer` receives a copy of all ICMP traffic on the
host's default interface. By the end you will have a working Docker Compose
stack and understand how to verify that mirroring is active.

## Prerequisites

- A Linux host (kernel ≥ 3.18) with Docker and Docker Compose installed.
- The `container-net-mirroring` image available locally or from the registry.

## Step 1 — Create the configuration file

```bash
cp config.example.yaml config.yaml
```

Open `config.yaml` and replace its contents with:

```yaml
mirrorings:
  - interface: default
    direction: ingress
    container: net-observer
    traffic: icmp
```

This rule mirrors all incoming ICMP packets on the default route interface into
the `net-observer` container.

## Step 2 — Start the observer container

```bash
docker run -d --name net-observer nicolaka/netshoot tcpdump -i any -n icmp
```

Keep a second terminal open to watch its output:

```bash
docker logs -f net-observer
```

## Step 3 — Apply mirroring rules

```bash
docker run --rm \
  --network=host \
  --cap-add=NET_ADMIN --cap-add=NET_RAW \
  -v /var/run:/var/run \
  -v /sys/class/net:/sys/class/net:ro \
  -v $(pwd)/config.yaml:/config.yaml:ro \
  ghcr.io/yannickland/container-net-mirroring:latest
```

You should see output like:

```
[mirrorings[0]] Resolving veth for container "net-observer" …
[mirrorings[0]] Container "net-observer" → veth veth3a1b2c4
[mirrorings[0]] Applying ingress mirroring on "eth0" → "veth3a1b2c4" (1 rule(s)) …
[mirrorings[0]] ✓ "ingress" mirroring active on "eth0"
All mirroring rules applied successfully.
```

## Step 4 — Generate test traffic

From another terminal, send a ping to any host:

```bash
ping -c 3 8.8.8.8
```

Switch to the `net-observer` log window — you should see the ICMP echo requests
arriving there as mirrored copies.

## Step 5 — Tear down

When you are done, remove the tc rules:

```bash
docker run --rm \
  --network=host \
  --cap-add=NET_ADMIN \
  -v /sys/class/net:/sys/class/net:ro \
  -v $(pwd)/config.yaml:/config.yaml:ro \
  ghcr.io/yannickland/container-net-mirroring:latest teardown --config /config.yaml
```

Then stop the observer container:

```bash
docker stop net-observer && docker rm net-observer
```

## What you achieved

- Created a `config.yaml` with a single ICMP mirroring rule.
- Started an observer container and verified that it receives mirrored packets.
- Applied and tore down tc rules using the `mirror apply` and `mirror teardown`
  commands.

**Next steps:**
- See [How to configure a custom traffic filter](../how-to-guides/how-to-custom-filter.md)
  for finer-grained control over what traffic is mirrored.
- Read the [Configuration file reference](../reference/configuration.md) for all
  available options.
- Read [Architecture and design](../explanation/architecture.md) to understand
  how the tc mirroring mechanism works.
