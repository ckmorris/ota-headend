// Copyright 2026 Chris Morris.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
// See LICENSE.md.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"ota-headend/internal/capture"
	"ota-headend/internal/devices/hdhomerun"
	"ota-headend/internal/headend"
)

var version = "dev"

const help = `OTA Headend — Chris Morris
Usage: ota-headend COMMAND [OPTIONS]

  init       Create configuration (never overwrites an existing file)
  doctor     Check installed tools and local configuration
  discover   List HDHomeRun devices and tuner state; read only
  scan       Scan RF channels on one explicitly selected idle tuner
  add        Append a scanned RF multiplex and its services to the shared config
  remove     Remove a multiplex from configuration
  validate   Validate configuration without touching hardware
  run        Capture all configured multiplexes and serve one M3U; Ctrl-C stops
  status     Show the running headend's last local status snapshot
  playlist   Print an M3U lineup for a player
  recover    Release this installation's saved tuner claims after a crash
  version    Print version

Use ota-headend COMMAND --help for options. Options follow the command.
Default configuration: ./config.json. Relative state/runtime paths are
resolved beside that file. Licensed under PolyForm Noncommercial 1.0.0; see LICENSE.md.
Legitimate use only; piracy is prohibited. Read EULA.md before running.
Starting the headend requires run --accept-terms to confirm your agreement.
`

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(out, help)
		return nil
	}
	command := args[0]
	switch command {
	case "init", "doctor", "discover", "scan", "add", "remove", "validate", "run", "status", "playlist", "recover":
	case "version":
		fmt.Fprintln(out, version)
		return nil
	default:
		return fmt.Errorf("unknown command %q; use ota-headend --help", command)
	}
	f := flag.NewFlagSet(command, flag.ContinueOnError)
	f.SetOutput(stderr)
	configPath := f.String("config", "config.json", "shared configuration file for all multiplexes")
	var device, name, scanPath, outputPath, rf, programs, base, listen *string
	var tuner *int
	var asJSON, acceptTerms *bool
	switch command {
	case "init":
		listen = f.String("listen", headend.DefaultConfig().Listen, "HTTP listen IP:port")
		base = f.String("base-url", "", "optional player-facing base URL")
	case "discover":
		asJSON = f.Bool("json", false, "print JSON")
	case "scan":
		device = f.String("device", "", "hardware ID from discover (required)")
		tuner = f.Int("tuner", -1, "idle tuner index (required)")
		rf = f.String("rf", "2-36", "RF list, such as 7,11,19-22")
		outputPath = f.String("output", "scan.json", "new scan result file; refuses overwrite")
	case "add":
		name = f.String("name", "", "unique multiplex name (required)")
		scanPath = f.String("scan", "scan.json", "saved scan result")
		rf = f.String("rf", "", "physical RF channel from scan (required)")
		device = f.String("device", "", "destination device (defaults to scanned device)")
		tuner = f.Int("tuner", -1, "destination idle tuner (defaults to scan tuner)")
		programs = f.String("programs", "", "comma-separated program numbers; default all clear services")
	case "remove":
		name = f.String("name", "", "multiplex name (required)")
	case "status":
		asJSON = f.Bool("json", false, "print JSON")
	case "playlist":
		base = f.String("base-url", "", "player-facing base URL, e.g. http://192.0.2.10:8090")
	case "run":
		acceptTerms = f.Bool("accept-terms", false, "agree to LICENSE.md and EULA.md, including legitimate use only and no piracy (required)")
	}
	if err := f.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if f.NArg() != 0 {
		return errors.New("unexpected positional arguments; options must follow the command")
	}
	if command == "run" {
		if !*acceptTerms {
			return errors.New("read LICENSE.md and EULA.md before running; you must agree to legitimate use only and no piracy. Re-run with --accept-terms to confirm your agreement")
		}
		fmt.Fprintln(stderr, "Terms accepted: legitimate use only; piracy and unlawful redistribution are prohibited. See EULA.md.")
	}
	if command == "init" {
		c := headend.DefaultConfig()
		c.Listen = *listen
		c.BaseURL = strings.TrimRight(*base, "/")
		if err := headend.CreateConfig(*configPath, c); err != nil {
			return err
		}
		fmt.Fprintln(out, "Created", *configPath, "— next: ota-headend doctor, then ota-headend discover")
		return nil
	}
	if command == "discover" {
		devices, err := hdhomerun.NewDiscoverer().Discover(ctx)
		if err != nil {
			return fmt.Errorf("discovery failed; run doctor and check LAN connectivity: %w", err)
		}
		if *asJSON {
			return writeJSON(out, devices)
		}
		if len(devices) == 0 {
			fmt.Fprintln(out, "No devices found. Check antenna tuner power, LAN and firewall; see docs/troubleshooting.md.")
			return nil
		}
		w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "DEVICE\tADDRESS\tMODEL\tTUNERS")
		for _, d := range devices {
			fmt.Fprintf(w, "%s\t%s\t%s\t%v\n", d.HardwareID, strings.Join(d.EndpointCandidates, ","), d.Model, d.Capabilities["tuner_count"])
			tuners, _ := d.State["tuners"].([]map[string]any)
			for _, t := range tuners {
				fmt.Fprintf(w, "  tuner%v\tchannel=%v\tlock=%v\tpps=%v\n", t["index"], t["channel"], t["lock"], t["pps"])
			}
		}
		return w.Flush()
	}
	if command == "doctor" {
		return doctor(*configPath, out)
	}
	c, err := headend.Load(*configPath)
	if err != nil {
		return fmt.Errorf("%w; create a new config with init if needed", err)
	}
	resolved := c.Resolve(*configPath)
	hw := headend.Hardware{Runner: hdhomerun.ExecRunner{Timeout: 5 * time.Second}, StateDir: resolved.StateDir}
	switch command {
	case "validate":
		fmt.Fprintf(out, "Valid configuration: %d multiplexes\n", len(c.Muxes))
		return nil
	case "playlist":
		url := *base
		if url == "" {
			url = c.BaseURL
		}
		if url == "" {
			endpoints, err := headend.PlaybackEndpoints(c.Listen)
			if err != nil {
				return fmt.Errorf("cannot determine playback address; specify --base-url: %w", err)
			}
			url = strings.TrimSuffix(endpoints[0].PlaylistURL, "/lineup.m3u")
		}
		if err := headend.ValidateBaseURL(url); err != nil {
			return err
		}
		fmt.Fprint(out, headend.Playlist(c, url))
		return nil
	case "status":
		var status headend.Status
		if err := headend.ReadJSON(filepath.Join(resolved.StateDir, "status.json"), &status); err != nil {
			return fmt.Errorf("no readable status; start the headend with run: %w", err)
		}
		stale := time.Since(status.UpdatedAt) > 15*time.Second
		if *asJSON {
			if err := writeJSON(out, status); err != nil {
				return err
			}
		} else {
			fmt.Fprintf(out, "Running: %v | snapshot: %s | stale: %v\n", status.Running, status.UpdatedAt.Local().Format(time.RFC3339), stale)
			if status.Running {
				if c.BaseURL != "" {
					fmt.Fprintln(out, "IPTV playlist:", strings.TrimRight(c.BaseURL, "/")+"/lineup.m3u")
				}
				for _, endpoint := range status.Playback {
					label := "LAN IP:"
					if net.ParseIP(endpoint.IP).IsLoopback() {
						label = "Local IP:"
					}
					fmt.Fprintln(out, label, endpoint.IP)
					fmt.Fprintln(out, "IPTV playlist:", endpoint.PlaylistURL)
				}
			}
			w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "MUX\tPROGRAM\tNAME\tREADY\tDETAIL")
			for _, o := range status.Outputs {
				fmt.Fprintf(w, "%s\t%d\t%s\t%v\t%s\n", o.Mux, o.Program, o.Name, o.Ready, o.Error)
			}
			_ = w.Flush()
		}
		if !status.Running || stale {
			return errors.New("headend is stopped or status is stale")
		}
		for _, o := range status.Outputs {
			if !o.Ready {
				return errors.New("one or more services are not ready")
			}
		}
		return nil
	}
	unlock, err := headend.Lock(resolved.StateDir)
	if err != nil {
		return err
	}
	defer unlock()
	// Reload after obtaining the lock so concurrent edits cannot lose an update.
	c, err = headend.Load(*configPath)
	if err != nil {
		return err
	}
	if c.Resolve(*configPath).StateDir != resolved.StateDir {
		return errors.New("state_dir changed while acquiring the lock; retry the command")
	}
	resolved = c.Resolve(*configPath)
	switch command {
	case "recover":
		if err := hw.Recover(ctx); err != nil {
			return err
		}
		fmt.Fprintln(out, "Saved claims recovered; unowned tuners were left untouched.")
		return nil
	case "scan":
		*device = strings.ToUpper(*device)
		if err := headend.ValidDevice(*device, *tuner); err != nil {
			return err
		}
		channels, err := headend.ParseRFList(*rf)
		if err != nil {
			return err
		}
		// Reserve the output before hardware access; never overwrite a user's file.
		file, err := os.OpenFile(*outputPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		success := false
		defer func() {
			_ = file.Close()
			if !success {
				_ = os.Remove(*outputPath)
			}
		}()
		if err := hw.Recover(ctx); err != nil {
			return err
		}
		result, err := hw.Scan(ctx, *device, *tuner, channels, stderr)
		if err != nil {
			return err
		}
		if err := writeJSON(file, result); err != nil {
			return err
		}
		if err := file.Sync(); err != nil {
			return err
		}
		success = true
		fmt.Fprintf(out, "Saved %d multiplexes to %s. Next: add --scan %s --rf RF --name NAME\n", len(result.Muxes), *outputPath, *outputPath)
		return nil
	case "add":
		var scan headend.ScanResult
		if err := headend.ReadJSON(*scanPath, &scan); err != nil {
			return err
		}
		if scan.Version != 1 {
			return errors.New("unsupported scan version")
		}
		rfNumber, err := strconv.Atoi(*rf)
		if err != nil {
			return errors.New("--rf is required and must be a physical channel from the scan")
		}
		m := headend.Mux{Name: *name, Device: strings.ToUpper(*device), Tuner: *tuner, RF: rfNumber}
		if m.Device == "" {
			m.Device = scan.Device
		}
		if m.Tuner < 0 {
			m.Tuner = scan.Tuner
		}
		selected := map[int]bool{}
		if *programs != "" {
			for _, raw := range strings.Split(*programs, ",") {
				n, e := strconv.Atoi(strings.TrimSpace(raw))
				if e != nil {
					return e
				}
				selected[n] = false
			}
		}
		for _, found := range scan.Muxes {
			if found.RF == rfNumber {
				for _, s := range found.Services {
					if _, ok := selected[s.Program]; len(selected) == 0 || ok {
						m.Services = append(m.Services, s)
						if ok {
							selected[s.Program] = true
						}
					}
				}
			}
		}
		for n, found := range selected {
			if !found {
				return fmt.Errorf("program %d was not found on RF %d", n, rfNumber)
			}
		}
		c.Muxes = append(c.Muxes, m)
		if err := c.Validate(); err != nil {
			return err
		}
		if err := headend.AtomicJSON(*configPath, c); err != nil {
			return err
		}
		fmt.Fprintf(out, "Added %s: RF %d, %d services. No capture started.\n", m.Name, m.RF, len(m.Services))
		return nil
	case "remove":
		if *name == "" {
			return errors.New("--name is required")
		}
		found := false
		var muxes []headend.Mux
		for _, m := range c.Muxes {
			if m.Name == *name {
				found = true
			} else {
				muxes = append(muxes, m)
			}
		}
		if !found {
			return fmt.Errorf("unknown mux %q", *name)
		}
		if muxes == nil {
			muxes = []headend.Mux{}
		}
		c.Muxes = muxes
		if err := headend.AtomicJSON(*configPath, c); err != nil {
			return err
		}
		fmt.Fprintln(out, "Removed", *name)
		return nil
	case "run":
		for _, name := range []string{"hdhomerun_config", "tsp"} {
			if _, err := exec.LookPath(name); err != nil {
				return fmt.Errorf("%s is missing; run doctor and follow README.md dependencies", name)
			}
		}
		logger := slog.New(slog.NewTextHandler(stderr, nil))
		engine := headend.Engine{Config: resolved, Hardware: hw, Capture: capture.ExecCaptureRunner{}, Logger: logger}
		return engine.Run(ctx)
	}
	return errors.New("unhandled command")
}

func writeJSON(w io.Writer, v any) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return e.Encode(v)
}
func doctor(path string, out io.Writer) error {
	failed := false
	for _, name := range []string{"hdhomerun_config", "tsp"} {
		found, err := exec.LookPath(name)
		if err != nil {
			fmt.Fprintf(out, "MISSING %s — see README.md dependencies\n", name)
			failed = true
		} else {
			fmt.Fprintf(out, "OK %s: %s\n", name, found)
		}
	}
	c, err := headend.Load(path)
	if err != nil {
		fmt.Fprintln(out, "CONFIG:", err)
		failed = true
	} else {
		fmt.Fprintf(out, "OK config: %d multiplexes, listener %s\n", len(c.Muxes), c.Listen)
		for _, dir := range []string{c.Resolve(path).StateDir, c.Resolve(path).RuntimeDir} {
			fmt.Fprintln(out, "Directory:", dir)
		}
	}
	if failed {
		return errors.New("resolve the missing dependencies or config above")
	}
	fmt.Fprintln(out, "Local checks passed. Use discover to check hardware; no tuner was changed.")
	return nil
}
