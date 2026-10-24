// Copyright 2026 Chris Morris.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
// See LICENSE.md.

package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"ota-headend/internal/headend"
)

func TestSetupFlowAndConfigurationRoundTrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	config := filepath.Join(dir, "config.json")
	scan := filepath.Join(dir, "scan.json")
	result := headend.ScanResult{Version: 1, Device: "1234ABCD", Tuner: 0, Muxes: []headend.ScannedMux{{RF: 7, FrequencyHz: 177000000, Services: []headend.Service{{Program: 3, Virtual: "5.1", Name: "Example TV"}, {Program: 4, Virtual: "5.2", Name: "Example Two"}}}}}
	if err := headend.AtomicJSON(scan, result); err != nil {
		t.Fatal(err)
	}
	commands := [][]string{
		{"init", "--config", config},
		{"add", "--config", config, "--scan", scan, "--rf", "7", "--name", "local", "--programs", "3"},
		{"validate", "--config", config},
		{"playlist", "--config", config, "--base-url", "http://192.0.2.10:8090"},
	}
	for _, args := range commands {
		var out bytes.Buffer
		if err := run(ctx, args, &out, &out); err != nil {
			t.Fatalf("%v: %v %s", args, err, out.String())
		}
		if args[0] == "playlist" && !strings.Contains(out.String(), "http://192.0.2.10:8090/hls/local/3/index.m3u8") {
			t.Fatal(out.String())
		}
	}
	c, err := headend.Load(config)
	if err != nil || len(c.Muxes) != 1 || len(c.Muxes[0].Services) != 1 {
		t.Fatal(c, err)
	}
	if c.Listen != "0.0.0.0:8090" {
		t.Fatal("setup did not enable LAN playback:", c.Listen)
	}
	var out bytes.Buffer
	if err := run(ctx, []string{"init", "--config", config}, &out, &out); err == nil {
		t.Fatal("init overwrote config")
	}
	if err := run(ctx, []string{"add", "--config", config, "--scan", scan, "--rf", "7", "--name", "second"}, &out, &out); err == nil {
		t.Fatal("double-booked a tuner")
	}
	if err := run(ctx, []string{"add", "--config", config, "--scan", scan, "--rf", "7", "--name", "second", "--tuner", "1", "--programs", "999"}, &out, &out); err == nil {
		t.Fatal("accepted unknown program")
	}
	if err := run(ctx, []string{"remove", "--config", config, "--name", "local"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	c, err = headend.Load(config)
	if err != nil || len(c.Muxes) != 0 {
		t.Fatal(c, err)
	}
	// User data lives beside the selected configuration, independent of cwd.
	if _, err := os.Stat(filepath.Join(dir, "state", "headend.lock")); err != nil {
		t.Fatal(err)
	}
}

func TestMultipleDevicesAppendToOneConfigAndShareOneLineup(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	config := filepath.Join(dir, "config.json")
	var out bytes.Buffer
	if err := run(ctx, []string{"init", "--config", config}, &out, &out); err != nil {
		t.Fatal(err)
	}
	for _, source := range []struct {
		device string
		tuner  int
		rf     int
		name   string
	}{
		{device: "1234ABCD", tuner: 2, rf: 14, name: "first"},
		{device: "1234ABCE", tuner: 0, rf: 9, name: "second"},
	} {
		services := []headend.Service{
			{Program: 3, Name: source.name + " main"},
			{Program: 4, Name: source.name + " subchannel"},
		}
		scan := filepath.Join(dir, source.name+"-scan.json")
		result := headend.ScanResult{Version: 1, Device: source.device, Tuner: source.tuner, Muxes: []headend.ScannedMux{{RF: source.rf, Services: services}}}
		if err := headend.AtomicJSON(scan, result); err != nil {
			t.Fatal(err)
		}
		args := []string{"add", "--config", config, "--scan", scan, "--rf", strconv.Itoa(source.rf), "--name", source.name}
		if err := run(ctx, args, &out, &out); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	c, err := headend.Load(config)
	if err != nil || len(c.Muxes) != 2 {
		t.Fatal("add did not preserve both multiplexes:", c, err)
	}
	if c.Muxes[0].Device != "1234ABCD" || c.Muxes[0].Tuner != 2 || c.Muxes[1].Device != "1234ABCE" || c.Muxes[1].Tuner != 0 {
		t.Fatal("lost scanned device/tuner assignments:", c.Muxes)
	}
	e := headend.Engine{Config: c}
	server := httptest.NewServer(e.Handler())
	defer server.Close()
	response, err := server.Client().Get(server.URL + "/lineup.m3u")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || strings.Count(string(data), "#EXTM3U") != 1 || strings.Count(string(data), "#EXTINF:") != 4 {
		t.Fatalf("expected one lineup with all four services: %s", data)
	}
	for _, path := range []string{"/hls/first/3/index.m3u8", "/hls/first/4/index.m3u8", "/hls/second/3/index.m3u8", "/hls/second/4/index.m3u8"} {
		if !strings.Contains(string(data), server.URL+path) {
			t.Fatalf("service missing from shared lineup: %s", path)
		}
	}
	out.Reset()
	if err := run(ctx, []string{"playlist", "--config", config, "--base-url", server.URL}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != string(data) {
		t.Fatal("exported and served lineups differ")
	}
}

func TestStatusDisplaysIPTVConnectionDetails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	c := headend.DefaultConfig()
	c.BaseURL = "https://tv.example/iptv"
	if err := headend.CreateConfig(path, c); err != nil {
		t.Fatal(err)
	}
	status := headend.Status{
		Version: 1, Running: true, UpdatedAt: time.Now().UTC(),
		Outputs:  []headend.OutputStatus{},
		Playback: []headend.PlaybackEndpoint{{IP: "192.168.1.20", PlaylistURL: "http://192.168.1.20:8090/lineup.m3u"}},
	}
	if err := headend.AtomicJSON(filepath.Join(dir, "state", "status.json"), status); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run(context.Background(), []string{"status", "--config", path}, &out, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"LAN IP: 192.168.1.20", "IPTV playlist: http://192.168.1.20:8090/lineup.m3u", "IPTV playlist: https://tv.example/iptv/lineup.m3u"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in %s", want, out.String())
		}
	}
}
