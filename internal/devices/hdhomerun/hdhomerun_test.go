// Copyright 2026 Chris Morris.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
// See LICENSE.md.

package hdhomerun

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

type fakeRunner map[string]string

func (f fakeRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	key := fmt.Sprintf("%s %v", name, args)
	if value, ok := f[key]; ok {
		return value, nil
	}
	return "", errors.New("missing fixture")
}

func TestParseDiscover(t *testing.T) {
	devices := ParseDiscover(`hdhomerun device 1234ABCD found at 2604:f6c0::1
hdhomerun device 1234ABCD found at 192.0.2.16
hdhomerun device 1234ABCE found at 192.0.2.30
garbage`)

	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %#v", devices)
	}
	if devices[0].ID != "1234ABCD" || devices[0].Address != "192.0.2.16" {
		t.Fatalf("unexpected first device %#v", devices[0])
	}
}

func TestParseTunerStatus(t *testing.T) {
	status := ParseTunerStatus("1234ABCD", 0, "ch=8vsb:599000000 lock=8vsb ss=82 snq=91 seq=100 bps=19251200 pps=1648 resource=192.0.2.24")

	expect := map[string]any{
		"index":           0,
		"identity":        "1234ABCD:tuner0",
		"channel":         "8vsb:599000000",
		"frequency_hz":    599000000,
		"lock":            "8vsb",
		"signal_strength": 82,
		"signal_quality":  91,
		"symbol_quality":  100,
		"bps":             19251200,
		"pps":             1648,
		"resource_lock":   "192.0.2.24",
	}
	if !reflect.DeepEqual(status, expect) {
		t.Fatalf("unexpected status %#v", status)
	}
}

func TestParseIdleTunerStatus(t *testing.T) {
	status := ParseTunerStatus("1234ABCE", 2, "ch=none lock=none ss=0 snq=0 seq=0 bps=0 pps=0")

	if status["identity"] != "1234ABCE:tuner2" || status["lock"] != "none" || status["bps"] != 0 {
		t.Fatalf("unexpected idle status %#v", status)
	}
}

func TestParseTunerStatusTargetResourceLock(t *testing.T) {
	status := ParseTunerStatus("1234ABCE", 3, "ch=8vsb:473000000 lock=8vsb target=udp://192.0.2.24:5000 ss=78 snq=100 seq=100")

	if status["resource_lock"] != "udp://192.0.2.24:5000" {
		t.Fatalf("expected target resource lock, got %#v", status)
	}
}

func TestParseStreamInfo(t *testing.T) {
	stream := ParseStreamInfo("TSID=0x0BC9\n3: 5.1 WCYB-DT")

	if stream["tsid"] != "0x0BC9" {
		t.Fatalf("unexpected stream info %#v", stream)
	}
	programs := stream["programs"].([]map[string]any)
	if programs[0]["program"] != 3 || programs[0]["virtual_channel"] != "5.1" || programs[0]["name"] != "WCYB-DT" {
		t.Fatalf("unexpected programs %#v", programs)
	}
}

func TestDiscovererUsesReadOnlyGetPaths(t *testing.T) {
	runner := fakeRunner{
		"hdhomerun_config [discover]":                        "hdhomerun device 1234ABCD found at 192.0.2.16",
		"hdhomerun_config [1234ABCD get /sys/model]":         "HDHomeRun CONNECT QUATRO",
		"hdhomerun_config [1234ABCD get /sys/version]":       "20250101",
		"hdhomerun_config [1234ABCD get /tuner0/status]":     "ch=8vsb:599000000 lock=8vsb ss=82 snq=91 seq=100 bps=19251200 pps=1648",
		"hdhomerun_config [1234ABCD get /tuner0/streaminfo]": "tsid=0x0BC9\n3: 5.1 WCYB-DT",
	}
	discoverer := Discoverer{Runner: runner, Command: "hdhomerun_config", MaxTuners: 1}

	observations, err := discoverer.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(observations) != 1 {
		t.Fatalf("expected one observation, got %#v", observations)
	}
	observation := observations[0]
	if observation.HardwareID != "1234ABCD" || observation.EndpointCandidates[0] != "192.0.2.16" {
		t.Fatalf("unexpected observation %#v", observation)
	}
	if observation.Capabilities["tuner_count"] != 1 {
		t.Fatalf("unexpected capabilities %#v", observation.Capabilities)
	}
}
