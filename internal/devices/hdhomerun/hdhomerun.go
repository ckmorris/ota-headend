// Copyright 2026 Chris Morris.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
// See LICENSE.md.

package hdhomerun

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"ota-headend/internal/devices"
)

const Kind = "hdhomerun"

type Runner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

type ExecRunner struct {
	Timeout time.Duration
}

func (r ExecRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, name, args...)
	out, err := cmd.CombinedOutput()
	if errors.Is(err, exec.ErrNotFound) {
		return "", err
	}
	if runCtx.Err() != nil {
		return "", runCtx.Err()
	}
	return strings.TrimSpace(string(out)), err
}

type Discoverer struct {
	Runner    Runner
	Command   string
	MaxTuners int
}

func NewDiscoverer() Discoverer {
	return Discoverer{Runner: ExecRunner{}, Command: "hdhomerun_config", MaxTuners: 8}
}

func (d Discoverer) Discover(ctx context.Context) ([]devices.Observation, error) {
	runner := d.Runner
	if runner == nil {
		runner = ExecRunner{}
	}
	command := d.Command
	if command == "" {
		command = "hdhomerun_config"
	}
	maxTuners := d.MaxTuners
	if maxTuners <= 0 {
		maxTuners = 8
	}

	output, err := runner.Run(ctx, command, "discover")
	if err != nil {
		return nil, err
	}
	discovered := ParseDiscover(output)
	observations := make([]devices.Observation, 0, len(discovered))
	for _, device := range discovered {
		observation := devices.Observation{
			Kind:               Kind,
			HardwareID:         device.ID,
			Manufacturer:       "SiliconDust",
			EndpointCandidates: []string{device.Address},
			Capabilities:       map[string]any{},
			State:              map[string]any{},
		}
		if model, err := get(ctx, runner, command, device.ID, "/sys/model"); err == nil && model != "" {
			observation.Model = model
		}
		if version, err := get(ctx, runner, command, device.ID, "/sys/version"); err == nil && version != "" {
			observation.Metadata = map[string]any{"firmware_version": version}
		}
		tuners := make([]map[string]any, 0, 4)
		for tuner := 0; tuner < maxTuners; tuner++ {
			status, err := get(ctx, runner, command, device.ID, fmt.Sprintf("/tuner%d/status", tuner))
			if err != nil {
				if tuner == 0 {
					continue
				}
				break
			}
			parsed := ParseTunerStatus(device.ID, tuner, status)
			if streaminfo, err := get(ctx, runner, command, device.ID, fmt.Sprintf("/tuner%d/streaminfo", tuner)); err == nil {
				if stream := ParseStreamInfo(streaminfo); len(stream) > 0 {
					parsed["stream"] = stream
				}
			}
			tuners = append(tuners, parsed)
		}
		observation.Capabilities["tuner_count"] = len(tuners)
		observation.State["tuners"] = tuners
		observations = append(observations, observation)
	}
	return observations, nil
}

func get(ctx context.Context, runner Runner, command string, deviceID string, path string) (string, error) {
	return runner.Run(ctx, command, deviceID, "get", path)
}

type Device struct {
	ID      string
	Address string
}

var discoverLine = regexp.MustCompile(`(?i)hdhomerun device ([0-9a-f]{8}) found at (\S+)`)

func ParseDiscover(output string) []Device {
	seen := map[string]Device{}
	for _, line := range strings.Split(output, "\n") {
		match := discoverLine.FindStringSubmatch(strings.TrimSpace(line))
		if len(match) != 3 {
			continue
		}
		id := strings.ToUpper(match[1])
		address := strings.TrimSpace(match[2])
		if ip := net.ParseIP(strings.Split(address, "%")[0]); ip == nil || ip.To4() == nil || ip.IsLoopback() || ip.IsUnspecified() {
			continue
		}
		if _, exists := seen[id]; !exists {
			seen[id] = Device{ID: id, Address: address}
		}
	}
	devices := make([]Device, 0, len(seen))
	for _, device := range seen {
		devices = append(devices, device)
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i].ID < devices[j].ID })
	return devices
}

func ParseTunerStatus(deviceID string, index int, status string) map[string]any {
	values := parseKeyValues(status)
	result := map[string]any{
		"index":    index,
		"identity": fmt.Sprintf("%s:tuner%d", strings.ToUpper(deviceID), index),
		"channel":  stringOrDefault(values["ch"], "none"),
		"lock":     stringOrDefault(values["lock"], "none"),
	}
	if frequency := frequencyFromChannel(values["ch"]); frequency > 0 {
		result["frequency_hz"] = frequency
	}
	addInt(result, "signal_strength", values["ss"])
	addInt(result, "signal_quality", values["snq"])
	addInt(result, "symbol_quality", values["seq"])
	addInt(result, "bps", values["bps"])
	addInt(result, "pps", values["pps"])
	if resourceLock := values["resource"]; resourceLock != "" {
		result["resource_lock"] = resourceLock
	} else if resourceLock := values["lockkey"]; resourceLock != "" {
		result["resource_lock"] = resourceLock
	} else if resourceLock := values["target"]; resourceLock != "" {
		result["resource_lock"] = resourceLock
	}
	return result
}

func ParseStreamInfo(output string) map[string]any {
	output = strings.TrimSpace(output)
	if output == "" || strings.EqualFold(output, "no data") {
		return nil
	}
	stream := map[string]any{}
	var programs []map[string]any
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if key, value, ok := strings.Cut(line, "="); ok && strings.EqualFold(key, "tsid") {
			stream["tsid"] = strings.TrimSpace(value)
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if program, err := strconv.Atoi(strings.TrimSuffix(fields[0], ":")); err == nil {
			entry := map[string]any{"program": program}
			if len(fields) > 1 {
				entry["virtual_channel"] = fields[1]
			}
			if len(fields) > 2 {
				entry["name"] = strings.Join(fields[2:], " ")
			}
			programs = append(programs, entry)
		}
	}
	if len(programs) > 0 {
		stream["programs"] = programs
	}
	return stream
}

func parseKeyValues(status string) map[string]string {
	values := map[string]string{}
	for _, field := range strings.Fields(status) {
		key, value, ok := strings.Cut(field, "=")
		if ok {
			values[key] = value
		}
	}
	return values
}

func stringOrDefault(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func frequencyFromChannel(channel string) int {
	_, raw, ok := strings.Cut(channel, ":")
	if !ok {
		return 0
	}
	frequency, _ := strconv.Atoi(raw)
	return frequency
}

func addInt(result map[string]any, key string, value string) {
	if value == "" {
		return
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		result[key] = parsed
	}
}
