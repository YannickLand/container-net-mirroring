# Test Coverage TODO

Tests for the following units require Linux kernel privileges or a live Docker
daemon and cannot be automated in the standard CI environment. Each entry
documents what would be verified and what is needed to implement it.

---

## TODO: iface.Resolve — "default" route resolution

**Feature / function:** `internal/iface.Resolve("default")`  
**Reason:** Calls `netlink.RouteListFiltered` which makes a real Netlink system
call. Requires a Linux host with an active default route and CAP_NET_ADMIN.  
**Would verify:** Returns the correct interface name for the default IPv4 route;
returns an error when no default route exists (mocked netlink or network
namespace).  
**Prerequisite:** Either a CAP_NET_ADMIN-enabled CI environment or a netlink
mock/interface injection to decouple the logic from the syscall.

---

## TODO: iface.Resolve — "loopback" resolution

**Feature / function:** `internal/iface.Resolve("loopback")`  
**Reason:** Calls `netlink.LinkList()` which requires a Linux Netlink socket.  
**Would verify:** Returns `["lo"]` on a standard Linux host; returns an error
when no loopback interface is found.  
**Prerequisite:** Same as above (CAP_NET_ADMIN or netlink mock).

---

## TODO: iface.Resolve — "all-physical" resolution

**Feature / function:** `internal/iface.Resolve("all-physical")`  
**Reason:** Reads `/sys/devices/virtual/net/` and calls Netlink to enumerate
physical interfaces — not reproducible without a real kernel.  
**Would verify:** Excludes loopback and virtual interfaces; returns at least one
physical interface name on a normal host.  
**Prerequisite:** CAP_NET_ADMIN or a controlled network-namespace environment.

---

## TODO: iface.Resolve — literal interface name

**Feature / function:** `internal/iface.Resolve("eth0")`  
**Reason:** Calls `netlink.LinkByName` which requires an actual kernel interface
to be present.  
**Would verify:** Returns `["eth0"]` when the interface exists; returns an
informative error when it does not.  
**Prerequisite:** CAP_NET_ADMIN or netlink mock.

---

## TODO: docker.VethName — happy path

**Feature / function:** `internal/docker.VethName(ctx, containerName)`  
**Reason:** Requires a running Docker daemon, a live container, and mounted
`/proc/<pid>/root/sys/class/net/eth0/iflink`. Cannot be replicated without a
Docker environment.  
**Would verify:** Returns the correct host-side veth interface name for a
running container; caches result correctly across calls in `runApply`.  
**Prerequisite:** A Docker-in-Docker or privileged CI runner with the Docker
socket available.

---

## TODO: docker.VethName — container not running

**Feature / function:** `internal/docker.VethName` error path (pid == 0)  
**Reason:** Requires Docker daemon to inspect a stopped container.  
**Would verify:** Returns an error containing "not running" when the container
exists but is stopped.  
**Prerequisite:** Docker daemon in CI.

---

## TODO: tc.Apply — installs qdisc and filter (integration)

**Feature / function:** `internal/tc.Apply(ifaceName, direction, vethName, rules)`  
**Reason:** Calls `netlink.QdiscAdd` and `netlink.FilterAdd` — real Netlink
operations that modify kernel state. Requires CAP_NET_ADMIN and real
interfaces.  
**Would verify:** After `Apply`, `netlink.QdiscList` returns the expected qdisc;
`netlink.FilterList` returns a filter with the correct match keys and mirred
action.  
**Prerequisite:** Privileged CI runner with a veth pair that can be created for
the test (e.g. using `ip link add`).

---

## TODO: tc.Teardown — removes qdisc idempotently

**Feature / function:** `internal/tc.Teardown(ifaceName, direction)`  
**Reason:** Same kernel-level requirement as `tc.Apply`.  
**Would verify:** After `Teardown`, no matching qdisc exists; calling `Teardown`
a second time does not return an error.  
**Prerequisite:** Same as `tc.Apply`.

---

## TODO: cmd/mirror apply — end-to-end CLI

**Feature / function:** `cmd/mirror.runApply(cfgFile)`  
**Reason:** Orchestrates config loading, Docker veth resolution, iface
resolution, and tc rule installation — every step requires kernel/Docker.  
**Would verify:** Exit code 0 on valid config; correct tc rules installed; error
message and non-zero exit code on missing container.  
**Prerequisite:** Privileged CI with Docker daemon and a running `net-observer`
container.

---

## TODO: cmd/mirror teardown — end-to-end CLI

**Feature / function:** `cmd/mirror.runTeardown(cfgFile)`  
**Reason:** Same as apply.  
**Would verify:** Exit code 0; qdiscs removed; idempotent on repeated calls.  
**Prerequisite:** Same as apply.
