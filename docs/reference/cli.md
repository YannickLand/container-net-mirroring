# CLI Reference

## Synopsis

```
mirror <command> [flags]
```

## Commands

### `mirror apply`

Reads the configuration file, validates all entries, resolves host interfaces
and container veth names, then installs tc qdiscs and filters.

Running `apply` on an already-configured host is safe: existing qdiscs are
reused and only missing filters are added.

**Flags:**

| Flag | Short | Default | Description |
|---|---|---|---|
| `--config` | `-c` | `config.yaml` | Path to the mirroring configuration file |

**Exit codes:**

| Code | Meaning |
|---|---|
| 0 | All rules applied successfully |
| 1 | Configuration validation error, container not found, or tc error |

**Example:**

```bash
mirror apply --config /config.yaml
```

---

### `mirror teardown`

Removes the tc qdiscs (and their associated filters) that were created by
`apply`. Reads the same configuration file to determine which interfaces and
directions to clean up.

Teardown is idempotent: running it when no rules are active is safe.

**Flags:**

| Flag | Short | Default | Description |
|---|---|---|---|
| `--config` | `-c` | `config.yaml` | Path to the mirroring configuration file |

**Exit codes:**

| Code | Meaning |
|---|---|
| 0 | All qdiscs removed (or were already absent) |
| 1 | Configuration validation error or netlink error |

**Example:**

```bash
mirror teardown --config /config.yaml
```

---

### `mirror --help`

Prints the help text for any command.

```bash
mirror --help
mirror apply --help
mirror teardown --help
```

## Global flags

| Flag | Short | Default | Description |
|---|---|---|---|
| `--config` | `-c` | `config.yaml` | Path to the mirroring configuration file (persistent across subcommands) |

## Requirements

The binary must run on a Linux host (kernel ≥ 3.18). It requires:

- `NET_ADMIN` capability
- `NET_RAW` capability
- `network_mode: host`
- `/var/run` mounted (Docker socket)
- `/sys/class/net` mounted (read-only)

## See also

- [Configuration file reference](configuration.md)
- [How to tear down mirroring rules](../how-to-guides/how-to-teardown.md)
