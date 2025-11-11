// Package tc manages Linux Traffic Control (tc) qdiscs and filters via the
// netlink API. It provides the primitives needed to set up and tear down
// traffic mirroring from a host interface to a container veth.
//
// # Byte-order convention for U32 filter keys
//
// The kernel's tc_u32_key.mask and tc_u32_key.val fields are __be32 (network /
// big-endian byte order). vishvananda/netlink serialises TcU32Key.Mask and
// TcU32Key.Val using the host's native byte order (little-endian on x86).
// Therefore all Val/Mask constants in this file are pre-swapped via swap32 so
// that the bytes reaching the kernel are in the correct big-endian order.
//
// # IPv4 header offset assumption
//
// Port and address matches use fixed offsets that assume a standard 20-byte
// IPv4 header (IHL=5, no options). This matches the behaviour of the iproute2
// tc command and is correct for the overwhelming majority of real traffic.
//
// # Qdisc model
//
//   - Ingress traffic  → ingress qdisc  (handle ffff:, parent HANDLE_INGRESS)
//   - Egress  traffic  → prio   qdisc   (handle   1:0, parent HANDLE_ROOT)
package tc

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	cfg "github.com/YannickLand/container-net-mirroring/internal/config"
)

// ——— handles ———————————————————————————————————————————————————————————————

var (
	handleIngress = netlink.MakeHandle(0xffff, 0) // ffff:0
	handleRoot1   = netlink.MakeHandle(1, 0)      // 1:0
)

// ——— public API ————————————————————————————————————————————————————————————

// Apply sets up the qdisc(s) and filter(s) on ifaceName to mirror traffic
// matching rule into the container veth (targetVeth), for the requested
// direction ("ingress", "egress", or "both").
func Apply(ifaceName, direction, targetVeth string, rules []cfg.FilterRule) error {
	src, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("tc: source interface %q not found: %w", ifaceName, err)
	}
	dst, err := netlink.LinkByName(targetVeth)
	if err != nil {
		return fmt.Errorf("tc: target veth %q not found: %w", targetVeth, err)
	}

	if direction == "ingress" || direction == "both" {
		if err := applyDirection(src, dst, "ingress", rules); err != nil {
			return err
		}
	}
	if direction == "egress" || direction == "both" {
		if err := applyDirection(src, dst, "egress", rules); err != nil {
			return err
		}
	}
	return nil
}

// Teardown removes the mirroring qdisc(s) from ifaceName for the given direction.
// Removing the qdisc automatically removes all attached filters.
func Teardown(ifaceName, direction string) error {
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("tc: interface %q not found: %w", ifaceName, err)
	}

	if direction == "ingress" || direction == "both" {
		if err := deleteQdisc(link, handleIngress, netlink.HANDLE_INGRESS); err != nil {
			return err
		}
	}
	if direction == "egress" || direction == "both" {
		if err := deleteQdisc(link, handleRoot1, netlink.HANDLE_ROOT); err != nil {
			return err
		}
	}
	return nil
}

// ——— internal helpers ——————————————————————————————————————————————————————

func applyDirection(src, dst netlink.Link, direction string, rules []cfg.FilterRule) error {
	var handle, parent uint32
	if direction == "ingress" {
		handle = handleIngress
		parent = netlink.HANDLE_INGRESS
	} else {
		handle = handleRoot1
		parent = netlink.HANDLE_ROOT
	}

	if err := ensureQdisc(src, handle, parent, direction); err != nil {
		return err
	}

	for i, rule := range rules {
		if err := addFilter(src, dst, handle, rule, uint16(i+1)); err != nil {
			return fmt.Errorf("tc: adding filter rule %d on %s/%s: %w",
				i+1, src.Attrs().Name, direction, err)
		}
	}
	return nil
}

// ensureQdisc adds the required qdisc if it is not already present.
func ensureQdisc(link netlink.Link, handle, parent uint32, direction string) error {
	existing, err := netlink.QdiscList(link)
	if err != nil {
		return fmt.Errorf("tc: cannot list qdiscs on %s: %w", link.Attrs().Name, err)
	}
	for _, q := range existing {
		if q.Attrs().Handle == handle {
			return nil // already present
		}
	}

	var qdisc netlink.Qdisc
	if direction == "ingress" {
		qdisc = &netlink.Ingress{
			QdiscAttrs: netlink.QdiscAttrs{
				LinkIndex: link.Attrs().Index,
				Handle:    handle,
				Parent:    parent,
			},
		}
	} else {
		qdisc = &netlink.Prio{
			QdiscAttrs: netlink.QdiscAttrs{
				LinkIndex: link.Attrs().Index,
				Handle:    handle,
				Parent:    parent,
			},
		}
	}

	if err := netlink.QdiscAdd(qdisc); err != nil {
		return fmt.Errorf("tc: cannot add %s qdisc on %s: %w", direction, link.Attrs().Name, err)
	}
	return nil
}

func deleteQdisc(link netlink.Link, handle, parent uint32) error {
	qdisc := &netlink.GenericQdisc{
		QdiscAttrs: netlink.QdiscAttrs{
			LinkIndex: link.Attrs().Index,
			Handle:    handle,
			Parent:    parent,
		},
		QdiscType: "ingress",
	}
	if err := netlink.QdiscDel(qdisc); err != nil {
		// Not an error if the qdisc was already absent (idempotent teardown).
		return fmt.Errorf("tc: cannot delete qdisc (handle %x) on %s: %w",
			handle, link.Attrs().Name, err)
	}
	return nil
}

// addFilter installs a U32 filter + mirred-mirror action on src that sends
// matching packets to dst.
func addFilter(src, dst netlink.Link, parent uint32, rule cfg.FilterRule, priority uint16) error {
	keys, nlProto := buildKeys(rule)

	filter := &netlink.U32{
		FilterAttrs: netlink.FilterAttrs{
			LinkIndex: src.Attrs().Index,
			Parent:    parent,
			Priority:  priority,
			Protocol:  nlProto,
		},
		Sel: &netlink.TcU32Sel{
			Flags: netlink.TC_U32_TERMINAL,
			Nkeys: uint8(len(keys)),
			Keys:  keys,
		},
		Actions: []netlink.Action{
			&netlink.MirredAction{
				ActionAttrs: netlink.ActionAttrs{
					Action: netlink.TC_ACT_PIPE,
				},
				MirredAction: netlink.TCA_EGRESS_MIRROR,
				Ifindex:      dst.Attrs().Index,
			},
		},
	}

	if err := netlink.FilterAdd(filter); err != nil {
		return fmt.Errorf("tc: FilterAdd: %w", err)
	}
	return nil
}

// buildKeys translates a FilterRule into the netlink U32 key slice and the
// Ethernet protocol constant (unix.ETH_P_ALL or unix.ETH_P_IP).
//
// Key Mask/Val are specified as big-endian (network byte order) integers and
// are pre-swapped for vishvananda/netlink's little-endian serialisation.
//
// IPv4 header layout used for offsets (20-byte standard header):
//
//	offset 0:  version/IHL/DSCP/ECN (1B each)
//	offset 8:  TTL(1B) | Protocol(1B) | Checksum(2B)
//	offset 12: Source IP (4B)
//	offset 16: Dest IP   (4B)
//	offset 20: Src Port (2B) | Dst Port (2B)   [TCP/UDP]
func buildKeys(rule cfg.FilterRule) ([]netlink.TcU32Key, uint16) {
	if rule.Protocol == "all" && rule.IPProto == 0 && rule.DstPort == 0 &&
		rule.SrcCIDR == "" && rule.DstCIDR == "" {
		// Match-all: u32 match 0 0
		return []netlink.TcU32Key{{Mask: 0, Val: 0}}, unix.ETH_P_ALL
	}

	var keys []netlink.TcU32Key

	if rule.IPProto != 0 {
		// IP protocol field is the second byte of the 32-bit word at offset 8
		// (bytes: TTL | Proto | Cksum_H | Cksum_L).
		// Big-endian word: 0x??pp???? where pp = proto → mask=0x00ff0000
		keys = append(keys, netlink.TcU32Key{
			Mask: swap32(0x00ff0000),
			Val:  swap32(uint32(rule.IPProto) << 16),
			Off:  8,
		})
	}

	if rule.DstPort != 0 {
		// The 32-bit word at offset 20 = [src_port(16b) | dst_port(16b)].
		// dst_port occupies the low 16 bits → mask=0x0000ffff
		keys = append(keys, netlink.TcU32Key{
			Mask: swap32(0x0000ffff),
			Val:  swap32(uint32(rule.DstPort)),
			Off:  20,
		})
	}

	if rule.SrcCIDR != "" {
		ip, mask, err := parseCIDRKey(rule.SrcCIDR)
		if err == nil {
			// Source IP is at offset 12 in the IPv4 header.
			keys = append(keys, netlink.TcU32Key{
				Mask: swap32(mask),
				Val:  swap32(ip),
				Off:  12,
			})
		}
	}

	if rule.DstCIDR != "" {
		ip, mask, err := parseCIDRKey(rule.DstCIDR)
		if err == nil {
			// Destination IP is at offset 16 in the IPv4 header.
			keys = append(keys, netlink.TcU32Key{
				Mask: swap32(mask),
				Val:  swap32(ip),
				Off:  16,
			})
		}
	}

	if len(keys) == 0 {
		keys = []netlink.TcU32Key{{Mask: 0, Val: 0}}
	}

	return keys, unix.ETH_P_IP
}

// swap32 converts a uint32 from big-endian to native (little-endian) byte
// order so that vishvananda/netlink's NativeEndian serialisation produces
// the correct __be32 bytes that the kernel expects.
func swap32(v uint32) uint32 {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], v)
	return binary.LittleEndian.Uint32(b[:])
}

// parseCIDRKey converts an IP-or-CIDR string into (ip_be32, mask_be32).
// Both returned values are big-endian 32-bit integers suitable for swap32.
func parseCIDRKey(s string) (uint32, uint32, error) {
	toUint32 := func(b []byte) uint32 {
		if len(b) < 4 {
			return 0
		}
		return binary.BigEndian.Uint32(b[:4])
	}

	if strings.Contains(s, "/") {
		_, ipNet, err := net.ParseCIDR(s)
		if err != nil {
			return 0, 0, err
		}
		return toUint32(ipNet.IP.To4()), toUint32(ipNet.Mask), nil
	}

	parsed := net.ParseIP(s).To4()
	if parsed == nil {
		return 0, 0, fmt.Errorf("not a valid IPv4 address: %s", s)
	}
	return toUint32(parsed), 0xffffffff, nil
}
