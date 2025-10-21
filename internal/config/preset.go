package config

import "sort"

// Preset describes a named traffic preset and its derived filter rules.
type Preset struct {
	// Description is a human-readable explanation shown in help text and warnings.
	Description string
	// OverPrivilegedWarning is non-empty when the preset captures more traffic
	// than a typical least-privilege deployment warrants.
	OverPrivilegedWarning string
	// Rules is the list of filter rules this preset expands into.
	// A preset may expand into more than one rule (e.g. "web" covers :80 and :443).
	Rules []FilterRule
}

// FilterRule is the resolved, kernel-level representation of a single tc filter.
type FilterRule struct {
	// Protocol is the Ethernet protocol: "ip" or "all".
	Protocol string
	// IPProto selects the IP-layer protocol (0 = any).
	// 1=ICMP, 6=TCP, 17=UDP.
	IPProto uint8
	// DstPort matches the destination port (0 = any).
	DstPort uint16
	// SrcCIDR matches the source address or CIDR (empty = any).
	SrcCIDR string
	// DstCIDR matches the destination address or CIDR (empty = any).
	DstCIDR string
}

// presets is the registry of all named presets.
var presets = map[string]Preset{
	"icmp": {
		Description: "ICMP traffic only (ping, traceroute)",
		Rules: []FilterRule{
			{Protocol: "ip", IPProto: 1},
		},
	},
	"web": {
		Description: "HTTP (TCP :80) and HTTPS (TCP :443)",
		Rules: []FilterRule{
			{Protocol: "ip", IPProto: 6, DstPort: 80},
			{Protocol: "ip", IPProto: 6, DstPort: 443},
		},
	},
	"dns": {
		Description: "DNS queries and responses (UDP and TCP :53)",
		Rules: []FilterRule{
			{Protocol: "ip", IPProto: 17, DstPort: 53},
			{Protocol: "ip", IPProto: 6, DstPort: 53},
		},
	},
	"ntp": {
		Description: "NTP time-sync traffic (UDP :123)",
		Rules: []FilterRule{
			{Protocol: "ip", IPProto: 17, DstPort: 123},
		},
	},
	"ssh": {
		Description: "SSH sessions (TCP :22)",
		Rules: []FilterRule{
			{Protocol: "ip", IPProto: 6, DstPort: 22},
		},
	},
	"all": {
		Description: "All traffic (no filtering)",
		OverPrivilegedWarning: "preset \"all\" mirrors every packet — " +
			"this violates the principle of least privilege. " +
			"Consider using a more specific preset or a custom filter.",
		Rules: []FilterRule{
			{Protocol: "all"},
		},
	},
}

// IsPreset reports whether name is a registered preset.
func IsPreset(name string) bool {
	_, ok := presets[name]
	return ok
}

// PresetNames returns a sorted list of all registered preset names.
func PresetNames() []string {
	names := make([]string, 0, len(presets))
	for k := range presets {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// ResolvePreset returns the Preset for the given name.
// Callers should check IsPreset first.
func ResolvePreset(name string) Preset {
	return presets[name]
}

// FilterRulesFor returns the resolved FilterRule slice for a Traffic value.
// It also returns any over-privilege warning string (non-empty = warn the user).
func FilterRulesFor(t Traffic) ([]FilterRule, string) {
	if t.Preset != "" {
		p := presets[t.Preset]
		return p.Rules, p.OverPrivilegedWarning
	}

	// Custom filter — translate to a single FilterRule.
	f := t.Custom
	rule := FilterRule{}

	switch f.Protocol {
	case "icmp":
		rule.Protocol = "ip"
		rule.IPProto = 1
	case "tcp":
		rule.Protocol = "ip"
		rule.IPProto = 6
	case "udp":
		rule.Protocol = "ip"
		rule.IPProto = 17
	default: // "all"
		rule.Protocol = "all"
	}

	if f.Port != nil {
		rule.DstPort = uint16(*f.Port)
	}
	rule.SrcCIDR = f.Src
	rule.DstCIDR = f.Dst

	return []FilterRule{rule}, ""
}
