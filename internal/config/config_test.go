package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/YannickLand/container-net-mirroring/internal/config"
)

// ——— helpers ——————————————————————————————————————————————————————————————

// writeConfig writes content to a temp file and returns its path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatalf("cannot create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("cannot write temp file: %v", err)
	}
	f.Close()
	return f.Name()
}

// ——— Load / validation ————————————————————————————————————————————————————

func TestLoad_ValidPreset(t *testing.T) {
	for _, preset := range config.PresetNames() {
		t.Run(preset, func(t *testing.T) {
			yaml := "mirrorings:\n" +
				"  - interface: default\n" +
				"    direction: ingress\n" +
				"    container: net-observer\n" +
				"    traffic: " + preset + "\n"
			cfg, err := config.Load(writeConfig(t, yaml))
			if err != nil {
				t.Fatalf("unexpected error for preset %q: %v", preset, err)
			}
			if len(cfg.Mirrorings) != 1 {
				t.Fatalf("expected 1 mirroring, got %d", len(cfg.Mirrorings))
			}
		})
	}
}

func TestLoad_ValidDirections(t *testing.T) {
	for _, dir := range []string{"ingress", "egress", "both"} {
		t.Run(dir, func(t *testing.T) {
			yaml := "mirrorings:\n" +
				"  - interface: default\n" +
				"    direction: " + dir + "\n" +
				"    container: c\n" +
				"    traffic: icmp\n"
			if _, err := config.Load(writeConfig(t, yaml)); err != nil {
				t.Fatalf("unexpected error for direction %q: %v", dir, err)
			}
		})
	}
}

func TestLoad_AllInterfaceTypes(t *testing.T) {
	for _, iface := range []string{"default", "loopback", "all-physical", "eth0"} {
		t.Run(iface, func(t *testing.T) {
			// Validation only checks that the field is non-empty, not that the
			// interface actually exists — resolution happens at apply time.
			yaml := "mirrorings:\n" +
				"  - interface: " + iface + "\n" +
				"    direction: ingress\n" +
				"    container: c\n" +
				"    traffic: icmp\n"
			if _, err := config.Load(writeConfig(t, yaml)); err != nil {
				t.Fatalf("unexpected error for interface %q: %v", iface, err)
			}
		})
	}
}

func TestLoad_ValidCustomFilter(t *testing.T) {
	port := 8080
	cases := []struct {
		name string
		yaml string
	}{
		{
			name: "protocol_only",
			yaml: "mirrorings:\n  - interface: default\n    direction: ingress\n    container: c\n    traffic:\n      protocol: tcp\n",
		},
		{
			name: "protocol_port",
			yaml: "mirrorings:\n  - interface: default\n    direction: ingress\n    container: c\n    traffic:\n      protocol: tcp\n      port: " + itoa(port) + "\n",
		},
		{
			name: "protocol_src_cidr",
			yaml: "mirrorings:\n  - interface: default\n    direction: ingress\n    container: c\n    traffic:\n      protocol: udp\n      src: 10.0.0.0/8\n",
		},
		{
			name: "protocol_dst_ip",
			yaml: "mirrorings:\n  - interface: default\n    direction: ingress\n    container: c\n    traffic:\n      protocol: icmp\n      dst: 192.168.1.1\n",
		},
		{
			name: "all_protocol",
			yaml: "mirrorings:\n  - interface: default\n    direction: ingress\n    container: c\n    traffic:\n      protocol: all\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := config.Load(writeConfig(t, tc.yaml)); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoad_InvalidCases(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name:    "empty_mirrorings",
			yaml:    "mirrorings: []\n",
			wantErr: "must not be empty",
		},
		{
			name:    "missing_interface",
			yaml:    "mirrorings:\n  - direction: ingress\n    container: c\n    traffic: icmp\n",
			wantErr: "interface must not be empty",
		},
		{
			name:    "missing_direction",
			yaml:    "mirrorings:\n  - interface: default\n    container: c\n    traffic: icmp\n",
			wantErr: "direction must not be empty",
		},
		{
			name:    "invalid_direction",
			yaml:    "mirrorings:\n  - interface: default\n    direction: sideways\n    container: c\n    traffic: icmp\n",
			wantErr: "invalid direction",
		},
		{
			name:    "missing_container",
			yaml:    "mirrorings:\n  - interface: default\n    direction: ingress\n    traffic: icmp\n",
			wantErr: "container must not be empty",
		},
		{
			name:    "unknown_preset",
			yaml:    "mirrorings:\n  - interface: default\n    direction: ingress\n    container: c\n    traffic: ftp\n",
			wantErr: "unknown traffic preset",
		},
		{
			name:    "custom_invalid_protocol",
			yaml:    "mirrorings:\n  - interface: default\n    direction: ingress\n    container: c\n    traffic:\n      protocol: gre\n",
			wantErr: "invalid value",
		},
		{
			name:    "custom_port_without_tcp_udp",
			yaml:    "mirrorings:\n  - interface: default\n    direction: ingress\n    container: c\n    traffic:\n      protocol: icmp\n      port: 80\n",
			wantErr: "port filtering is only valid for tcp or udp",
		},
		{
			name:    "custom_invalid_port",
			yaml:    "mirrorings:\n  - interface: default\n    direction: ingress\n    container: c\n    traffic:\n      protocol: tcp\n      port: 99999\n",
			wantErr: "not a valid port number",
		},
		{
			name:    "custom_invalid_src",
			yaml:    "mirrorings:\n  - interface: default\n    direction: ingress\n    container: c\n    traffic:\n      protocol: tcp\n      src: not-an-ip\n",
			wantErr: "not a valid",
		},
		{
			name:    "custom_invalid_dst_cidr",
			yaml:    "mirrorings:\n  - interface: default\n    direction: ingress\n    container: c\n    traffic:\n      protocol: tcp\n      dst: 999.0.0.0/8\n",
			wantErr: "not a valid",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := config.Load(writeConfig(t, tc.yaml))
			if err == nil {
				t.Fatal("expected an error but got nil")
			}
			if tc.wantErr != "" && !containsSubstr(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestLoad_NonExistentFile(t *testing.T) {
	_, err := config.Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoad_MalformedYAML(t *testing.T) {
	_, err := config.Load(writeConfig(t, "mirrorings: [\nunclosed"))
	if err == nil {
		t.Fatal("expected error for malformed YAML, got nil")
	}
}

// ——— Preset registry ——————————————————————————————————————————————————————

func TestIsPreset(t *testing.T) {
	known := []string{"icmp", "dns", "ntp", "web", "ssh", "all"}
	for _, name := range known {
		if !config.IsPreset(name) {
			t.Errorf("IsPreset(%q) = false, want true", name)
		}
	}
	unknown := []string{"", "ftp", "ALL", "TCP", "custom"}
	for _, name := range unknown {
		if config.IsPreset(name) {
			t.Errorf("IsPreset(%q) = true, want false", name)
		}
	}
}

func TestPresetNames_Sorted(t *testing.T) {
	names := config.PresetNames()
	if len(names) == 0 {
		t.Fatal("PresetNames returned empty slice")
	}
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("PresetNames not sorted: %v", names)
		}
	}
}

// ——— FilterRulesFor ———————————————————————————————————————————————————————

func TestFilterRulesFor_Preset(t *testing.T) {
	cases := []struct {
		preset    string
		wantRules int
		wantWarn  bool
	}{
		{"icmp", 1, false},
		{"dns", 2, false},  // UDP :53 + TCP :53
		{"ntp", 1, false},
		{"web", 2, false},  // :80 + :443
		{"ssh", 1, false},
		{"all", 1, true},   // emits over-privilege warning
	}

	for _, tc := range cases {
		t.Run(tc.preset, func(t *testing.T) {
			traffic := config.Traffic{Preset: tc.preset}
			rules, warn := config.FilterRulesFor(traffic)
			if len(rules) != tc.wantRules {
				t.Errorf("preset %q: got %d rules, want %d", tc.preset, len(rules), tc.wantRules)
			}
			if tc.wantWarn && warn == "" {
				t.Errorf("preset %q: expected a warning, got none", tc.preset)
			}
			if !tc.wantWarn && warn != "" {
				t.Errorf("preset %q: unexpected warning %q", tc.preset, warn)
			}
		})
	}
}

func TestFilterRulesFor_Custom(t *testing.T) {
	port := 8080
	cases := []struct {
		name         string
		filter       config.Filter
		wantProto    string // FilterRule.Protocol field
		wantIPProto  uint8
		wantDstPort  uint16
	}{
		{
			name:        "tcp",
			filter:      config.Filter{Protocol: "tcp"},
			wantProto:   "ip",
			wantIPProto: 6,
		},
		{
			name:        "udp",
			filter:      config.Filter{Protocol: "udp"},
			wantProto:   "ip",
			wantIPProto: 17,
		},
		{
			name:        "icmp",
			filter:      config.Filter{Protocol: "icmp"},
			wantProto:   "ip",
			wantIPProto: 1,
		},
		{
			name:        "all",
			filter:      config.Filter{Protocol: "all"},
			wantProto:   "all",
			wantIPProto: 0,
		},
		{
			name:        "tcp_with_port",
			filter:      config.Filter{Protocol: "tcp", Port: &port},
			wantProto:   "ip",
			wantIPProto: 6,
			wantDstPort: 8080,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			traffic := config.Traffic{Custom: &tc.filter}
			rules, warn := config.FilterRulesFor(traffic)
			if warn != "" {
				t.Errorf("unexpected warning for custom filter: %q", warn)
			}
			if len(rules) != 1 {
				t.Fatalf("expected 1 rule, got %d", len(rules))
			}
			r := rules[0]
			if r.Protocol != tc.wantProto {
				t.Errorf("Protocol: got %q, want %q", r.Protocol, tc.wantProto)
			}
			if r.IPProto != tc.wantIPProto {
				t.Errorf("IPProto: got %d, want %d", r.IPProto, tc.wantIPProto)
			}
			if r.DstPort != tc.wantDstPort {
				t.Errorf("DstPort: got %d, want %d", r.DstPort, tc.wantDstPort)
			}
		})
	}
}

func TestFilterRulesFor_CustomCIDR(t *testing.T) {
	traffic := config.Traffic{Custom: &config.Filter{
		Protocol: "tcp",
		Src:      "10.0.0.0/8",
		Dst:      "192.168.1.1",
	}}
	rules, _ := config.FilterRulesFor(traffic)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].SrcCIDR != "10.0.0.0/8" {
		t.Errorf("SrcCIDR: got %q, want %q", rules[0].SrcCIDR, "10.0.0.0/8")
	}
	if rules[0].DstCIDR != "192.168.1.1" {
		t.Errorf("DstCIDR: got %q, want %q", rules[0].DstCIDR, "192.168.1.1")
	}
}

// ——— Traffic YAML unmarshaling ————————————————————————————————————————————

func TestTraffic_UnmarshalString(t *testing.T) {
	yaml := "mirrorings:\n  - interface: default\n    direction: ingress\n    container: c\n    traffic: dns\n"
	cfg, err := config.Load(writeConfig(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := cfg.Mirrorings[0]
	if m.Traffic.Preset != "dns" {
		t.Errorf("Preset: got %q, want %q", m.Traffic.Preset, "dns")
	}
	if m.Traffic.Custom != nil {
		t.Error("Custom should be nil for a preset")
	}
}

func TestTraffic_UnmarshalMapping(t *testing.T) {
	yaml := "mirrorings:\n  - interface: default\n    direction: ingress\n    container: c\n    traffic:\n      protocol: udp\n      port: 53\n"
	cfg, err := config.Load(writeConfig(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := cfg.Mirrorings[0]
	if m.Traffic.Preset != "" {
		t.Errorf("Preset should be empty, got %q", m.Traffic.Preset)
	}
	if m.Traffic.Custom == nil {
		t.Fatal("Custom should not be nil for a mapping")
	}
	if m.Traffic.Custom.Protocol != "udp" {
		t.Errorf("Protocol: got %q, want %q", m.Traffic.Custom.Protocol, "udp")
	}
	if m.Traffic.Custom.Port == nil || *m.Traffic.Custom.Port != 53 {
		t.Errorf("Port: got %v, want 53", m.Traffic.Custom.Port)
	}
}

// ——— helpers ——————————————————————————————————————————————————————————————

func containsSubstr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
