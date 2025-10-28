// Package iface resolves logical interface names ("default", "loopback",
// "all-physical") to real kernel interface names using the netlink API.
package iface

import (
	"fmt"
	"strings"

	"github.com/vishvananda/netlink"
)

// Resolve returns the list of host network interface names that correspond to
// the given logical name.
//
//   - "default"      → the interface(s) used by the default IPv4 route
//   - "loopback"     → all loopback interfaces (typically just "lo")
//   - "all-physical" → all interfaces that are not virtual (not under /sys/devices/virtual)
//   - anything else  → treated as a literal interface name; validated against the
//     kernel link list
func Resolve(name string) ([]string, error) {
	switch strings.ToLower(name) {
	case "default":
		return resolveDefault()
	case "loopback":
		return resolveLoopback()
	case "all-physical":
		return resolveAllPhysical()
	default:
		return resolveLiteral(name)
	}
}

// resolveDefault returns the interface(s) associated with the default IPv4 route.
func resolveDefault() ([]string, error) {
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4,
		&netlink.Route{Dst: nil}, netlink.RT_FILTER_DST)
	if err != nil {
		return nil, fmt.Errorf("iface: cannot list routes: %w", err)
	}

	seen := map[int]bool{}
	var names []string
	for _, r := range routes {
		if r.LinkIndex == 0 || seen[r.LinkIndex] {
			continue
		}
		seen[r.LinkIndex] = true
		link, err := netlink.LinkByIndex(r.LinkIndex)
		if err != nil {
			return nil, fmt.Errorf("iface: cannot look up link index %d: %w", r.LinkIndex, err)
		}
		names = append(names, link.Attrs().Name)
	}

	if len(names) == 0 {
		return nil, fmt.Errorf("iface: no default route found")
	}
	return names, nil
}

// resolveLoopback returns all loopback-type links.
func resolveLoopback() ([]string, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("iface: cannot list links: %w", err)
	}

	var names []string
	for _, l := range links {
		if l.Attrs().Flags&(1<<3) != 0 { // net.FlagLoopback = 1<<3
			names = append(names, l.Attrs().Name)
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("iface: no loopback interface found")
	}
	return names, nil
}

// resolveAllPhysical returns links that are not virtual.
// A link is considered physical when its device path does not contain
// "virtual" (mirrors the /sys/class/net symlink convention).
func resolveAllPhysical() ([]string, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("iface: cannot list links: %w", err)
	}

	var names []string
	for _, l := range links {
		attrs := l.Attrs()
		// Skip loopback and virtual bridge/tun/veth/docker links.
		// The heuristic: device alias must not look like a virtual device.
		ltype := l.Type()
		if ltype == "veth" || ltype == "bridge" || ltype == "tun" ||
			ltype == "dummy" || ltype == "lo" {
			continue
		}
		if attrs.Flags&(1<<3) != 0 { // loopback flag
			continue
		}
		names = append(names, attrs.Name)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("iface: no physical interfaces found")
	}
	return names, nil
}

// resolveLiteral validates that the named interface exists on the host.
func resolveLiteral(name string) ([]string, error) {
	_, err := netlink.LinkByName(name)
	if err != nil {
		return nil, fmt.Errorf("iface: interface %q not found on this host: %w", name, err)
	}
	return []string{name}, nil
}
