// Package config loads and validates the mirroring configuration file.
package config

import (
	"fmt"
	"net"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the top-level structure of config.yaml.
type Config struct {
	Mirrorings []Mirroring `yaml:"mirrorings"`
}

// Mirroring describes one traffic-forwarding rule.
type Mirroring struct {
	// Interface is the host-side network interface to mirror traffic from.
	// Accepted values: "default", "loopback", "all-physical", or a literal
	// interface name (e.g. "eth0").
	Interface string `yaml:"interface"`

	// Direction specifies which traffic direction to capture.
	// Accepted values: "ingress", "egress", "both".
	Direction string `yaml:"direction"`

	// Container is the name of the target Docker container (exact match,
	// as it appears in "docker ps").
	Container string `yaml:"container"`

	// Traffic is either a preset name (e.g. "icmp") or a custom filter block.
	Traffic Traffic `yaml:"traffic"`
}

// Traffic holds either a named preset or a custom filter specification.
// In YAML it can appear as a plain string or as a mapping.
type Traffic struct {
	Preset string  // non-empty when a string preset was given
	Custom *Filter // non-nil when a custom filter block was given
}

// Filter is the custom traffic filter specification.
type Filter struct {
	// Protocol selects the L4 protocol. Accepted: "tcp", "udp", "icmp", "all".
	Protocol string `yaml:"protocol"`
	// Port matches the destination port (valid only for "tcp" or "udp").
	Port *int `yaml:"port,omitempty"`
	// Src matches the source IP address or CIDR block (e.g. "10.0.0.0/8").
	Src string `yaml:"src,omitempty"`
	// Dst matches the destination IP address or CIDR block.
	Dst string `yaml:"dst,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler so that Traffic can be either
// a bare string ("icmp") or a mapping (custom filter block).
func (t *Traffic) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		t.Preset = strings.TrimSpace(value.Value)
	case yaml.MappingNode:
		var f Filter
		if err := value.Decode(&f); err != nil {
			return fmt.Errorf("traffic: failed to decode custom filter: %w", err)
		}
		t.Custom = &f
	default:
		return fmt.Errorf("traffic: expected a string preset or a mapping, got node kind %v", value.Kind)
	}
	return nil
}

// Load reads and validates a configuration file at the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read config file %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("cannot parse config file %q: %w", path, err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if len(c.Mirrorings) == 0 {
		return fmt.Errorf("config: mirrorings list must not be empty")
	}
	for i, m := range c.Mirrorings {
		if err := m.validate(i); err != nil {
			return err
		}
	}
	return nil
}

func (m *Mirroring) validate(idx int) error {
	pos := fmt.Sprintf("mirrorings[%d]", idx)

	if m.Interface == "" {
		return fmt.Errorf("%s: interface must not be empty", pos)
	}

	switch m.Direction {
	case "ingress", "egress", "both":
	case "":
		return fmt.Errorf("%s: direction must not be empty (ingress | egress | both)", pos)
	default:
		return fmt.Errorf("%s: invalid direction %q — must be ingress, egress, or both", pos, m.Direction)
	}

	if m.Container == "" {
		return fmt.Errorf("%s: container must not be empty", pos)
	}

	if m.Traffic.Preset == "" && m.Traffic.Custom == nil {
		return fmt.Errorf("%s: traffic must be a preset name or a custom filter block", pos)
	}

	if m.Traffic.Preset != "" {
		if !IsPreset(m.Traffic.Preset) {
			return fmt.Errorf("%s: unknown traffic preset %q — valid presets: %s",
				pos, m.Traffic.Preset, strings.Join(PresetNames(), ", "))
		}
	}

	if m.Traffic.Custom != nil {
		if err := m.Traffic.Custom.validate(pos); err != nil {
			return err
		}
	}

	return nil
}

var validProtocols = map[string]bool{
	"tcp": true, "udp": true, "icmp": true, "all": true,
}

func (f *Filter) validate(pos string) error {
	proto := strings.ToLower(f.Protocol)
	if !validProtocols[proto] {
		return fmt.Errorf("%s.traffic.protocol: invalid value %q — must be tcp, udp, icmp, or all", pos, f.Protocol)
	}
	if f.Port != nil {
		if proto != "tcp" && proto != "udp" {
			return fmt.Errorf("%s.traffic.port: port filtering is only valid for tcp or udp, not %q", pos, proto)
		}
		if *f.Port < 1 || *f.Port > 65535 {
			return fmt.Errorf("%s.traffic.port: %d is not a valid port number (1–65535)", pos, *f.Port)
		}
	}
	if f.Src != "" {
		if err := validateIPOrCIDR(f.Src); err != nil {
			return fmt.Errorf("%s.traffic.src: %w", pos, err)
		}
	}
	if f.Dst != "" {
		if err := validateIPOrCIDR(f.Dst); err != nil {
			return fmt.Errorf("%s.traffic.dst: %w", pos, err)
		}
	}
	return nil
}

func validateIPOrCIDR(s string) error {
	if strings.Contains(s, "/") {
		_, _, err := net.ParseCIDR(s)
		if err != nil {
			return fmt.Errorf("%q is not a valid CIDR: %w", s, err)
		}
	} else {
		if net.ParseIP(s) == nil {
			return fmt.Errorf("%q is not a valid IP address", s)
		}
	}
	return nil
}
