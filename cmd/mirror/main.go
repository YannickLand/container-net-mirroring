//go:build linux

// mirror – configurable host-to-container traffic mirroring tool.
//
// Usage:
//
//	mirror apply   [--config <file>]   # set up mirroring rules
//	mirror teardown [--config <file>]  # remove mirroring rules
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/YannickLand/container-net-mirroring/internal/config"
	"github.com/YannickLand/container-net-mirroring/internal/docker"
	"github.com/YannickLand/container-net-mirroring/internal/iface"
	"github.com/YannickLand/container-net-mirroring/internal/tc"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

// ——— root command ——————————————————————————————————————————————————————————

func rootCmd() *cobra.Command {
	var cfgFile string

	root := &cobra.Command{
		Use:   "mirror",
		Short: "Configurable host-to-container traffic mirroring",
		Long: `mirror enables a Docker container to observe host-level network traffic
without the unsafe network_mode: host setting, by configuring Linux Traffic
Control (tc) mirroring rules from the host into the container's veth interface.

Requires: NET_ADMIN and NET_RAW capabilities, access to /var/run (Docker socket)
and /sys/class/net.`,
	}

	root.PersistentFlags().StringVarP(&cfgFile, "config", "c", "config.yaml",
		"path to the mirroring configuration file")

	root.AddCommand(applyCmd(&cfgFile))
	root.AddCommand(teardownCmd(&cfgFile))

	return root
}

// ——— apply ————————————————————————————————————————————————————————————————

func applyCmd(cfgFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "apply",
		Short: "Apply mirroring rules from the configuration file",
		Long: `Apply reads the configuration file, validates all entries, resolves host
interfaces and container veth names, then installs the tc qdiscs and filters
that mirror matching traffic into each target container.

Running apply on an already-configured host is safe: existing qdiscs are
reused and only missing filters are added.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApply(*cfgFile)
		},
	}
}

func runApply(cfgFile string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return err
	}

	ctx := context.Background()

	// Cache of container name → veth to avoid repeated Docker exec calls.
	vethCache := map[string]string{}

	for i, m := range cfg.Mirrorings {
		pos := fmt.Sprintf("mirrorings[%d]", i)

		// Resolve host interface(s).
		ifaces, err := iface.Resolve(m.Interface)
		if err != nil {
			return fmt.Errorf("%s: %w", pos, err)
		}

		// Resolve veth (cached per container).
		veth, ok := vethCache[m.Container]
		if !ok {
			fmt.Printf("[%s] Resolving veth for container %q …\n", pos, m.Container)
			veth, err = docker.VethName(ctx, m.Container)
			if err != nil {
				return fmt.Errorf("%s: %w", pos, err)
			}
			vethCache[m.Container] = veth
			fmt.Printf("[%s] Container %q → veth %q\n", pos, m.Container, veth)
		}

		// Resolve filter rules.
		rules, warning := config.FilterRulesFor(m.Traffic)
		if warning != "" {
			fmt.Fprintf(os.Stderr, "WARNING [%s]: %s\n", pos, warning)
		}

		// Apply to each resolved interface.
		for _, ifName := range ifaces {
			fmt.Printf("[%s] Applying %s mirroring on %q → %q (%d rule(s)) …\n",
				pos, m.Direction, ifName, veth, len(rules))
			if err := tc.Apply(ifName, m.Direction, veth, rules); err != nil {
				return fmt.Errorf("%s: %w", pos, err)
			}
			fmt.Printf("[%s] ✓ %q mirroring active on %q\n", pos, m.Direction, ifName)
		}
	}

	fmt.Println("All mirroring rules applied successfully.")
	return nil
}

// ——— teardown —————————————————————————————————————————————————————————————

func teardownCmd(cfgFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "teardown",
		Short: "Remove mirroring rules defined in the configuration file",
		Long: `Teardown removes the tc qdiscs (and their associated filters) that were
created by apply. It reads the same configuration file to know which
interfaces and directions to clean up.

Teardown is idempotent: running it when no rules are active is safe.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeardown(*cfgFile)
		},
	}
}

func runTeardown(cfgFile string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return err
	}

	for i, m := range cfg.Mirrorings {
		pos := fmt.Sprintf("mirrorings[%d]", i)

		ifaces, err := iface.Resolve(m.Interface)
		if err != nil {
			// Best-effort teardown: log and continue.
			fmt.Fprintf(os.Stderr, "WARNING [%s]: cannot resolve interface %q: %v\n",
				pos, m.Interface, err)
			continue
		}

		for _, ifName := range ifaces {
			fmt.Printf("[%s] Removing %s qdisc from %q …\n", pos, m.Direction, ifName)
			if err := tc.Teardown(ifName, m.Direction); err != nil {
				// Log and continue — teardown is best-effort.
				fmt.Fprintf(os.Stderr, "WARNING [%s]: %v\n", pos, err)
			} else {
				fmt.Printf("[%s] ✓ %q cleaned up on %q\n", pos, m.Direction, ifName)
			}
		}
	}

	fmt.Println("Teardown complete.")
	return nil
}
