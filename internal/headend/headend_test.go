// Copyright 2026 Chris Morris.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
// See LICENSE.md.

package headend

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"ota-headend/internal/capture"
)

func TestFrequencyAndRFList(t *testing.T) {
	for rf, want := range map[int]int{2: 57, 4: 69, 5: 79, 6: 85, 7: 177, 13: 213, 14: 473, 36: 605} {
		got, err := Frequency(rf)
		if err != nil || got != want*1_000_000 {
			t.Fatalf("RF%d=%d, %v", rf, got, err)
		}
	}
	for _, raw := range []string{"1", "37", "36-2", "2-999999", "x", "2,"} {
		if _, err := ParseRFList(raw); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	got, err := ParseRFList("7,5-7,36")
	if err != nil || !reflect.DeepEqual(got, []int{5, 6, 7, 36}) {
		t.Fatal(got, err)
	}
}

func sampleMux() Mux {
	return Mux{Name: "local", Device: "1234ABCD", Tuner: 0, RF: 7, Services: []Service{{Program: 3, Virtual: "5.1", Name: "Example TV"}}}
}
func TestConfigRejectsAmbiguousAndUnsafeBindings(t *testing.T) {
	tests := []Mux{sampleMux(), sampleMux(), sampleMux(), sampleMux(), sampleMux()}
	tests[0].Name = "../outside"
	tests[1].Device = "FFFFFFFF"
	tests[2].RF = 37
	tests[3].Tuner = -1
	tests[4].Services = []Service{{Program: 0}}
	for _, m := range tests {
		c := DefaultConfig()
		c.Muxes = []Mux{m}
		if c.Validate() == nil {
			t.Fatalf("accepted %+v", m)
		}
	}
	c := DefaultConfig()
	c.Muxes = []Mux{sampleMux(), sampleMux()}
	c.Muxes[1].Name = "other"
	if c.Validate() == nil {
		t.Fatal("same tuner assigned twice")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"secret_typo":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("ignored unknown configuration")
	}
	c = DefaultConfig()
	if err := CreateConfig(path, c); err == nil {
		t.Fatal("overwrote existing config")
	}
	resolved := c.Resolve(path)
	if resolved.StateDir != filepath.Join(dir, "state") {
		t.Fatal(resolved)
	}
}

type fakeRunner struct {
	values map[string]string
	calls  []string
	fail   string
}

func (r *fakeRunner) Run(_ context.Context, _ string, args ...string) (string, error) {
	call := strings.Join(args, " ")
	r.calls = append(r.calls, call)
	if strings.Contains(call, r.fail) && r.fail != "" {
		return "", errors.New("denied")
	}
	if value, ok := r.values[call]; ok {
		return value, nil
	}
	if len(args) > 2 && args[1] == "get" {
		return "", errors.New("missing fixture")
	}
	return "", nil
}
func idleHardware(t *testing.T) (Hardware, *fakeRunner) {
	t.Helper()
	r := &fakeRunner{values: map[string]string{
		"1234ABCD get /tuner0/lockkey": "none",
		"1234ABCD get /tuner0/target":  "none",
		"1234ABCD get /tuner0/status":  "ch=none lock=none bps=0 pps=0",
	}}
	return Hardware{Runner: r, StateDir: t.TempDir()}, r
}
func TestBusyOrIncompleteTunerNeverReceivesASet(t *testing.T) {
	for _, change := range []struct{ field, value string }{
		{"lockkey", "192.0.2.12"}, {"target", "udp://192.0.2.12:5000"},
		{"status", "ch=8vsb:177000000 lock=8vsb bps=19251200 pps=100"},
		{"status", "ch=none lock=none"},
	} {
		h, r := idleHardware(t)
		r.values["1234ABCD get /tuner0/"+change.field] = change.value
		if _, err := h.Acquire(context.Background(), "1234ABCD", 0); err == nil {
			t.Fatal("claimed busy or unknown tuner")
		}
		for _, call := range r.calls {
			if strings.Contains(call, " set ") {
				t.Fatal("changed a busy tuner", call)
			}
		}
	}
}
func TestOwnershipRecoveryAndCleanup(t *testing.T) {
	h, r := idleHardware(t)
	c, err := h.Acquire(context.Background(), "1234ABCD", 0)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(h.claimPath(c))
	if info.Mode().Perm() != 0600 {
		t.Fatal("claim not private")
	}
	r.fail = "set /tuner0/target"
	if err := h.Release(context.Background(), c); err == nil {
		t.Fatal("ignored cleanup failure")
	}
	if _, err := os.Stat(h.claimPath(c)); err != nil {
		t.Fatal("lost ownership after failed release")
	}
	r.fail = ""
	r.values["1234ABCD get /tuner0/lockkey"] = "192.0.2.10"
	r.calls = nil
	if err := h.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	fields := []string{"lockkey " + c.Key, "target none", "filter none", "channel none", "lockkey none"}
	if len(r.calls) != 6 {
		t.Fatal(r.calls)
	}
	for i, field := range fields {
		if !strings.HasSuffix(r.calls[i+1], field) {
			t.Fatal("wrong recovery order", r.calls)
		}
	}
	if _, err := os.Stat(h.claimPath(c)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("claim survived successful cleanup")
	}
}
func TestRecoveryRefusesForeignOrCorruptClaims(t *testing.T) {
	h, r := idleHardware(t)
	c, err := h.Acquire(context.Background(), "1234ABCD", 0)
	if err != nil {
		t.Fatal(err)
	}
	r.calls = nil
	r.values["1234ABCD get /tuner0/lockkey"] = "192.0.2.99"
	r.fail = "set /tuner0/lockkey"
	if err := h.Recover(context.Background()); err == nil {
		t.Fatal("accepted foreign lock")
	}
	for _, call := range r.calls {
		if strings.Contains(call, "target") || strings.Contains(call, "channel") {
			t.Fatal("changed foreign tuner")
		}
	}
	if err := os.WriteFile(h.claimPath(c), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	r.calls = nil
	if h.Recover(context.Background()) == nil || len(r.calls) != 0 {
		t.Fatal("corrupt record touched device")
	}
}
func TestResetRecoveryDoesNotTouchUnownedTuner(t *testing.T) {
	h, r := idleHardware(t)
	c, err := h.Acquire(context.Background(), "1234ABCD", 0)
	if err != nil {
		t.Fatal(err)
	}
	r.calls = nil
	if err := h.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(r.calls) != 1 || strings.Contains(r.calls[0], "set") {
		t.Fatal(r.calls)
	}
	if _, err := os.Stat(h.claimPath(c)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("reset claim not removed")
	}
}
func TestProcessLock(t *testing.T) {
	dir := t.TempDir()
	unlock, err := Lock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := Lock(dir); err == nil {
		second()
		t.Fatal("concurrent process entered")
	}
	unlock()
	unlock, err = Lock(dir)
	if err != nil {
		t.Fatal(err)
	}
	unlock()
}
func writePlaylist(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := os.WriteFile(filepath.Join(filepath.Dir(path), fmt.Sprintf("seg_%d.ts", i)), []byte("synthetic test transport"), 0640); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(path, []byte("#EXTM3U\n#EXTINF:4,\nseg_0.ts\n#EXTINF:4,\nseg_1.ts\n"), 0640); err != nil {
		t.Fatal(err)
	}
}
func TestHTTPRestrictsFilesAndRejectsStaleHLS(t *testing.T) {
	c := DefaultConfig()
	c.RuntimeDir = t.TempDir()
	c.Muxes = []Mux{sampleMux()}
	e := Engine{Config: c}
	path := e.path(c.Muxes[0], c.Muxes[0].Services[0])
	writePlaylist(t, path)
	e.status = Status{Running: true, Outputs: []OutputStatus{{Path: ServicePath(c.Muxes[0], c.Muxes[0].Services[0]), Ready: true}}}
	handler := e.Handler()
	for target, want := range map[string]int{"/lineup.m3u": 200, "/healthz": 200, "/hls/local/3/index.m3u8": 200, "/hls/local/3/seg_0.ts": 200, "/hls/local/3/": 404, "/hls/local/3/private.json": 404, "/hls/../state/claims/x.json": 404, "/hls/other/3/index.m3u8": 404} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", target, nil))
		if w.Code != want {
			t.Fatalf("%s: %d, want %d", target, w.Code, want)
		}
	}
	old := time.Now().Add(-time.Minute)
	_ = os.Chtimes(path, old, old)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/hls/local/3/index.m3u8", nil))
	if w.Code != 503 {
		t.Fatal("served stale playlist", w.Code)
	}
}
func TestReadinessRequiresSegments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.m3u8")
	writePlaylist(t, path)
	if !ReadyPlaylist(path) {
		t.Fatal("valid fixture not ready")
	}
	_ = os.Remove(filepath.Join(filepath.Dir(path), "seg_1.ts"))
	if ReadyPlaylist(path) {
		t.Fatal("missing segment was ready")
	}
}
func TestScanFailureStillReleasesOwnedTuner(t *testing.T) {
	h, r := idleHardware(t)
	r.fail = "set /tuner0/channel 8vsb:"
	_, err := h.Scan(context.Background(), "1234ABCD", 0, []int{7}, io.Discard)
	if err == nil {
		t.Fatal("scan failure ignored")
	}
	if !strings.HasSuffix(r.calls[len(r.calls)-1], "set /tuner0/lockkey none") {
		t.Fatal("did not release after scan error", r.calls)
	}
	entries, _ := os.ReadDir(filepath.Join(h.StateDir, "claims"))
	if len(entries) != 0 {
		t.Fatal("claim not cleaned up")
	}
}

type fakeProcess struct{ done chan error }

func (p *fakeProcess) Done() <-chan error         { return p.done }
func (p *fakeProcess) Stop(context.Context) error { p.done <- nil; return nil }

type fakeCapture struct{ request capture.CaptureRequest }

func (f *fakeCapture) Start(_ context.Context, r capture.CaptureRequest) (capture.Process, error) {
	f.request = r
	for _, o := range r.Outputs {
		dir := filepath.Dir(o.PlaylistPath)
		for _, name := range []string{"seg_0.ts", "seg_1.ts"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte("synthetic data"), 0640); err != nil {
				return nil, err
			}
		}
		if err := os.WriteFile(o.PlaylistPath, []byte("#EXTM3U\n#EXTINF:4,\nseg_0.ts\n#EXTINF:4,\nseg_1.ts\n"), 0640); err != nil {
			return nil, err
		}
	}
	return &fakeProcess{done: make(chan error, 1)}, nil
}

func TestEngineLifecycleServesHealthySiblingAndReleasesClaim(t *testing.T) {
	h, r := idleHardware(t)
	r.values["1234ABCD get /tuner0/streaminfo"] = "3: 5.1 Example TV"
	c := DefaultConfig()
	c.RuntimeDir = t.TempDir()
	c.StateDir = h.StateDir
	c.Muxes = []Mux{sampleMux()}
	c.Muxes[0].Services = append(c.Muxes[0].Services, Service{Program: 4, Name: "Absent"})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	c.Listen = listener.Addr().String()
	_ = listener.Close()
	f := &fakeCapture{}
	var logs bytes.Buffer
	e := Engine{Config: c, Hardware: h, Capture: f, Logger: slog.New(slog.NewTextHandler(&logs, nil))}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- e.Run(ctx) }()
	client := http.Client{Timeout: time.Second}
	deadline := time.Now().Add(5 * time.Second)
	for {
		response, err := client.Get("http://" + c.Listen + "/hls/local/3/index.m3u8")
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == 200 {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("healthy service never became available", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	response, err := client.Get("http://" + c.Listen + "/hls/local/4/index.m3u8")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != 503 {
		t.Fatal("absent program was advertised as ready")
	}
	var running Status
	if err := ReadJSON(filepath.Join(h.StateDir, "status.json"), &running); err != nil {
		t.Fatal(err)
	}
	wantURL := "http://" + c.Listen + "/lineup.m3u"
	if len(running.Playback) != 1 || running.Playback[0].IP != "127.0.0.1" || running.Playback[0].PlaylistURL != wantURL {
		t.Fatal("running status missing playback address:", running.Playback)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	// HTTP shutdown itself may take five seconds before capture cleanup starts.
	case <-time.After(10 * time.Second):
		t.Fatal("shutdown hung")
	}
	if !strings.Contains(logs.String(), "IPTV playlist") || !strings.Contains(logs.String(), wantURL) {
		t.Fatal("startup did not display the player URL:", logs.String())
	}
	if len(f.request.Outputs) != 1 || f.request.Outputs[0].ProgramNumber != 3 {
		t.Fatal("absent service sent to extractor", f.request)
	}
	entries, _ := os.ReadDir(filepath.Join(h.StateDir, "claims"))
	if len(entries) != 0 {
		t.Fatal("tuner claim not released")
	}
	var status Status
	if err := ReadJSON(filepath.Join(h.StateDir, "status.json"), &status); err != nil || status.Running {
		t.Fatal("shutdown status wrong", status, err)
	}
}
