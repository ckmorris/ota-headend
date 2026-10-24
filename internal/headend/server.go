// Copyright 2026 Chris Morris.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
// See LICENSE.md.

package headend

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"ota-headend/internal/capture"
)

type OutputStatus struct {
	Mux     string `json:"mux"`
	Program int    `json:"program"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Ready   bool   `json:"ready"`
	Error   string `json:"error,omitempty"`
}
type Status struct {
	Version   int                `json:"version"`
	UpdatedAt time.Time          `json:"updated_at"`
	Running   bool               `json:"running"`
	Outputs   []OutputStatus     `json:"outputs"`
	Playback  []PlaybackEndpoint `json:"playback,omitempty"`
}
type Engine struct {
	Config   Config
	Hardware Hardware
	Capture  capture.CaptureRunner
	Logger   *slog.Logger
	mu       sync.RWMutex
	status   Status
	playback []PlaybackEndpoint
}
type runningMux struct {
	mux     Mux
	claim   Claim
	process capture.Process
	errors  map[int]string
}

func (e *Engine) root() string { return filepath.Join(e.Config.RuntimeDir, "hls") }
func (e *Engine) path(m Mux, s Service) string {
	return filepath.Join(e.root(), m.Name, strconv.Itoa(s.Program), "index.m3u8")
}

func (e *Engine) start(ctx context.Context, m Mux) (r *runningMux, err error) {
	claim, err := e.Hardware.Acquire(ctx, m.Device, m.Tuner)
	if err != nil {
		return nil, err
	}
	r = &runningMux{mux: m, claim: claim, errors: map[int]string{}}
	defer func() {
		if err != nil {
			cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			err = errors.Join(err, e.Hardware.Release(cleanup, claim))
		}
	}()
	freq, _ := Frequency(m.RF)
	if err = e.Hardware.set(ctx, claim, "channel", fmt.Sprintf("8vsb:%d", freq)); err != nil {
		return nil, err
	}
	if err = e.Hardware.set(ctx, claim, "filter", "0x0000-0x1FFF"); err != nil {
		return nil, err
	}
	// Wait for a fresh program table before deciding a service is absent.
	var present []Service
	for i := 0; i < 10; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
		info, getErr := e.Hardware.get(ctx, m.Device, m.Tuner, "streaminfo")
		if getErr == nil {
			present = Services(info)
		}
		if len(present) > 0 {
			break
		}
	}
	if len(present) == 0 {
		return nil, fmt.Errorf("mux %s: no clear services found on RF %d; check antenna and rescan", m.Name, m.RF)
	}
	programs := map[int]bool{}
	for _, s := range present {
		programs[s.Program] = true
	}
	var outputs []capture.CaptureOutput
	for _, s := range m.Services {
		dir := filepath.Dir(e.path(m, s))
		if err = os.MkdirAll(dir, 0750); err != nil {
			return nil, err
		}
		if !programs[s.Program] {
			r.errors[s.Program] = "program absent from current transport; rescan or restart after it returns"
			continue
		}
		outputs = append(outputs, capture.CaptureOutput{ProgramNumber: s.Program, PlaylistPath: e.path(m, s), SegmentTemplate: filepath.Join(dir, fmt.Sprintf("seg_%d_00000.ts", time.Now().UnixNano()))})
	}
	if len(outputs) == 0 {
		return nil, fmt.Errorf("mux %s: none of the configured services is present", m.Name)
	}
	r.process, err = e.Capture.Start(ctx, capture.CaptureRequest{DeviceID: m.Device, TunerIndex: m.Tuner, LockKey: claim.Key, Outputs: outputs})
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (e *Engine) observe(all []*runningMux) Status {
	status := Status{Version: 1, UpdatedAt: time.Now().UTC(), Running: true, Outputs: []OutputStatus{}, Playback: e.playback}
	for _, r := range all {
		if r.process != nil {
			select {
			case err := <-r.process.Done():
				message := "capture exited; restart the headend to retry"
				if err != nil {
					message = err.Error()
				}
				for _, s := range r.mux.Services {
					r.errors[s.Program] = message
				}
				r.process = nil
			default:
			}
		}
		failures := map[string]string{}
		if p, ok := r.process.(interface{ OutputFailures() map[string]string }); ok {
			failures = p.OutputFailures()
		}
		for _, s := range r.mux.Services {
			message := r.errors[s.Program]
			if m := failures[e.path(r.mux, s)]; m != "" {
				message = m
			}
			ready := message == "" && r.process != nil && ReadyPlaylist(e.path(r.mux, s))
			if !ready && message == "" {
				message = "waiting for fresh HLS segments"
			}
			status.Outputs = append(status.Outputs, OutputStatus{Mux: r.mux.Name, Program: s.Program, Name: s.Name, Path: ServicePath(r.mux, s), Ready: ready, Error: message})
		}
	}
	return status
}

// Readiness requires a recent playlist and actual nonempty local segments.
func ReadyPlaylist(path string) bool {
	info, err := os.Stat(path)
	if err != nil || time.Since(info.ModTime()) > 20*time.Second {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.HasPrefix(string(data), "#EXTM3U") {
		return false
	}
	segments := 0
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if filepath.Base(line) != line || !strings.HasSuffix(line, ".ts") {
			return false
		}
		stat, err := os.Stat(filepath.Join(filepath.Dir(path), line))
		if err != nil || stat.Size() == 0 {
			return false
		}
		segments++
	}
	return segments >= 2
}

func (e *Engine) Handler() http.Handler {
	fs := http.FileServer(http.Dir(e.root()))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		switch r.URL.Path {
		case "/healthz":
			e.mu.RLock()
			status := e.status
			e.mu.RUnlock()
			healthy := status.Running && len(status.Outputs) > 0
			for _, o := range status.Outputs {
				healthy = healthy && o.Ready
			}
			if !healthy {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
			fmt.Fprintln(w, "headend running:", status.Running)
			return
		case "/lineup.m3u":
			w.Header().Set("Content-Type", "audio/x-mpegurl")
			base := e.Config.BaseURL
			if base == "" {
				// The accepted socket's address is reachable by this client, even
				// on a multihomed host. Do not trust Host or forwarded headers.
				if address, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr); ok {
					base = "http://" + address.String()
				}
			}
			fmt.Fprint(w, Playlist(e.Config, base))
			return
		}
		if strings.Contains(r.URL.Path, "..") || strings.Contains(r.URL.Path, "\\") {
			http.NotFound(w, r)
			return
		}
		allowed := false
		for _, m := range e.Config.Muxes {
			for _, s := range m.Services {
				dir := strings.TrimSuffix(ServicePath(m, s), "index.m3u8")
				if !strings.HasPrefix(r.URL.Path, dir) {
					continue
				}
				name := strings.TrimPrefix(r.URL.Path, dir)
				if name != "index.m3u8" && !(filepath.Base(name) == name && strings.HasPrefix(name, "seg_") && strings.HasSuffix(name, ".ts")) {
					continue
				}
				if name == "index.m3u8" {
					e.mu.RLock()
					ready := false
					for _, o := range e.status.Outputs {
						if o.Path == r.URL.Path {
							ready = o.Ready
						}
					}
					e.mu.RUnlock()
					if !ready || !ReadyPlaylist(e.path(m, s)) {
						http.Error(w, "service not ready; inspect ota-headend status", http.StatusServiceUnavailable)
						return
					}
					w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
				} else {
					w.Header().Set("Content-Type", "video/mp2t")
				}
				allowed = true
			}
		}
		if !allowed {
			http.NotFound(w, r)
			return
		}
		http.StripPrefix("/hls/", fs).ServeHTTP(w, r)
	})
}

// Run holds tuner claims until shutdown. Failed muxes stay visible in status;
// retry is an explicit restart so an outage cannot turn into a takeover loop.
func (e *Engine) Run(ctx context.Context) (err error) {
	if e.Logger == nil {
		e.Logger = slog.Default()
	}
	if len(e.Config.Muxes) == 0 {
		return errors.New("no muxes configured; run scan, then add")
	}
	if err := e.Hardware.Recover(ctx); err != nil {
		return err
	}
	// Bind before tuning, so a port conflict cannot disturb a tuner.
	listener, err := net.Listen("tcp", e.Config.Listen)
	if err != nil {
		return err
	}
	defer listener.Close()
	e.playback, err = PlaybackEndpoints(e.Config.Listen)
	if err != nil {
		e.Logger.Warn("could not list playback addresses", "error", err)
	}
	if err := os.MkdirAll(e.root(), 0750); err != nil {
		return err
	}
	children, cancelChildren := context.WithCancel(ctx)
	var all []*runningMux
	statusPath := filepath.Join(e.Config.StateDir, "status.json")
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		for _, r := range all {
			if r.process != nil {
				_ = r.process.Stop(cleanup)
			}
			if r.claim.Key != "" {
				err = errors.Join(err, e.Hardware.Release(cleanup, r.claim))
			}
		}
		cancelChildren()
		stopped := Status{Version: 1, UpdatedAt: time.Now().UTC(), Running: false, Outputs: []OutputStatus{}}
		err = errors.Join(err, AtomicJSON(statusPath, stopped))
	}()
	for _, m := range e.Config.Muxes {
		// Remove only this validated mux's transient media, never durable state.
		if err := os.RemoveAll(filepath.Join(e.root(), m.Name)); err != nil {
			return err
		}
		r, startErr := e.start(children, m)
		if startErr != nil {
			// Keep other muxes running. Failed ownership cleanup is preserved on disk.
			e.Logger.Error("mux could not start", "mux", m.Name, "error", startErr)
			r = &runningMux{mux: m, errors: map[int]string{}}
			for _, s := range m.Services {
				r.errors[s.Program] = startErr.Error()
			}
		}
		all = append(all, r)
		if ctx.Err() != nil {
			return nil
		}
	}
	publish := func() error {
		status := e.observe(all)
		e.mu.Lock()
		e.status = status
		e.mu.Unlock()
		return AtomicJSON(statusPath, status)
	}
	if err := publish(); err != nil {
		return err
	}
	server := &http.Server{Handler: e.Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	serverErr := make(chan error, 1)
	go func() { serverErr <- server.Serve(listener) }()
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	e.Logger.Info("headend listening", "address", e.Config.Listen, "lineup", "/lineup.m3u")
	if e.Config.BaseURL != "" {
		e.Logger.Info("IPTV playlist", "url", strings.TrimRight(e.Config.BaseURL, "/")+"/lineup.m3u")
	}
	for _, endpoint := range e.playback {
		label := "LAN IP"
		if net.ParseIP(endpoint.IP).IsLoopback() {
			label = "Local IP"
		}
		e.Logger.Info(label, "address", endpoint.IP)
		e.Logger.Info("IPTV playlist — load this URL in your IPTV player", "url", endpoint.PlaylistURL)
	}
	e.Logger.Info("Preparing services; waiting for playable HLS segments")
	readiness := startupReadiness{}
	readiness.update(e.Logger, e.status)
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-serverErr:
			return err
		case <-tick.C:
			if err := publish(); err != nil {
				return err
			}
			readiness.update(e.Logger, e.status)
		}
	}
}
