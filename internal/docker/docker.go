// Package docker resolves a container name to its veth interface name on the
// host by querying the Docker daemon via the official Go SDK.
package docker

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	dockerclient "github.com/docker/docker/client"
	"github.com/vishvananda/netlink"
)

// VethName returns the name of the host-side veth interface that is paired
// with the eth0 interface inside the named container.
//
// Strategy:
//  1. ContainerInspect → container PID
//  2. Read /proc/<pid>/root/sys/class/net/eth0/iflink — the host-side link
//     index is readable through the container's procfs root without entering
//     the network namespace.
//  3. Resolve the index to an interface name via netlink on the host.
func VethName(ctx context.Context, containerName string) (string, error) {
	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return "", fmt.Errorf("docker: cannot create client: %w", err)
	}
	defer cli.Close()

	info, err := cli.ContainerInspect(ctx, containerName)
	if err != nil {
		return "", fmt.Errorf("docker: cannot inspect container %q: %w", containerName, err)
	}

	pid := info.State.Pid
	if pid == 0 {
		return "", fmt.Errorf("docker: container %q is not running (pid=0)", containerName)
	}

	// Read the host-side veth index through the container's procfs root.
	// This works without entering the network namespace because /proc/<pid>/root
	// is accessible from the host as long as /proc is mounted (which it always is)
	// and we have sufficient privileges (root / CAP_SYS_PTRACE or CAP_NET_ADMIN).
	iflinkPath := fmt.Sprintf("/proc/%d/root/sys/class/net/eth0/iflink", pid)
	raw, err := os.ReadFile(iflinkPath)
	if err != nil {
		return "", fmt.Errorf(
			"docker: cannot read iflink for container %q (pid %d) at %s: %w",
			containerName, pid, iflinkPath, err)
	}

	ifIndex, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return "", fmt.Errorf(
			"docker: unexpected iflink value %q for container %q: %w",
			strings.TrimSpace(string(raw)), containerName, err)
	}

	// Resolve the host-side link by its index.
	link, err := netlink.LinkByIndex(ifIndex)
	if err != nil {
		return "", fmt.Errorf(
			"docker: cannot find host interface with index %d (veth peer of container %q): %w",
			ifIndex, containerName, err)
	}

	return link.Attrs().Name, nil
}
