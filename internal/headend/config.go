// Copyright 2026 Chris Morris.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
// See LICENSE.md.

package headend

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Config struct {
	Version    int    `json:"version"`
	Listen     string `json:"listen"`
	BaseURL    string `json:"base_url,omitempty"`
	StateDir   string `json:"state_dir"`
	RuntimeDir string `json:"runtime_dir"`
	Muxes      []Mux  `json:"muxes"`
}

type Mux struct {
	Name     string    `json:"name"`
	Device   string    `json:"device"`
	Tuner    int       `json:"tuner"`
	RF       int       `json:"rf"`
	Services []Service `json:"services"`
}

type Service struct {
	Program int    `json:"program"`
	Virtual string `json:"virtual_channel,omitempty"`
	Name    string `json:"name"`
}

var slug = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
var deviceID = regexp.MustCompile(`^[0-9A-F]{8}$`)

func DefaultConfig() Config {
	return Config{Version: 1, Listen: "0.0.0.0:8090", StateDir: "state", RuntimeDir: "run", Muxes: []Mux{}}
}

// RF is the physical US ATSC 1.0 channel, not a station's virtual number.
func Frequency(rf int) (int, error) {
	switch {
	case rf >= 2 && rf <= 4:
		return (57 + (rf-2)*6) * 1_000_000, nil
	case rf >= 5 && rf <= 6:
		return (79 + (rf-5)*6) * 1_000_000, nil
	case rf >= 7 && rf <= 13:
		return (177 + (rf-7)*6) * 1_000_000, nil
	case rf >= 14 && rf <= 36:
		return (473 + (rf-14)*6) * 1_000_000, nil
	default:
		return 0, fmt.Errorf("RF channel must be between 2 and 36")
	}
}

func ValidDevice(device string, tuner int) error {
	if !deviceID.MatchString(device) || device == "FFFFFFFF" || device == "00000000" {
		return errors.New("device must be a specific eight-digit HDHomeRun hardware ID from discover")
	}
	if tuner < 0 || tuner > 7 {
		return errors.New("tuner must be between 0 and 7")
	}
	return nil
}

func (c Config) Validate() error {
	if c.Version != 1 {
		return errors.New("unsupported config version; expected 1")
	}
	host, port, err := net.SplitHostPort(c.Listen)
	if err != nil {
		return errors.New("listen must be an IP address and port, for example 0.0.0.0:8090")
	}
	p, err := strconv.Atoi(port)
	if net.ParseIP(host) == nil || err != nil || p < 1 || p > 65535 {
		return errors.New("listen must contain a valid IP address and port")
	}
	if c.StateDir == "" || c.RuntimeDir == "" {
		return errors.New("state_dir and runtime_dir are required")
	}
	if c.BaseURL != "" {
		if err := ValidateBaseURL(c.BaseURL); err != nil {
			return err
		}
	}
	names, tuners := map[string]bool{}, map[string]bool{}
	for _, m := range c.Muxes {
		if !slug.MatchString(m.Name) || names[m.Name] {
			return fmt.Errorf("invalid or duplicate mux name %q; use lowercase letters, digits, dash or underscore", m.Name)
		}
		names[m.Name] = true
		if err := ValidDevice(m.Device, m.Tuner); err != nil {
			return fmt.Errorf("mux %s: %w", m.Name, err)
		}
		binding := fmt.Sprintf("%s:%d", m.Device, m.Tuner)
		if tuners[binding] {
			return fmt.Errorf("tuner %s is assigned more than once", binding)
		}
		tuners[binding] = true
		if _, err := Frequency(m.RF); err != nil {
			return err
		}
		if len(m.Services) == 0 {
			return fmt.Errorf("mux %s has no services", m.Name)
		}
		programs := map[int]bool{}
		for _, s := range m.Services {
			if s.Program < 1 || s.Program > 65535 || programs[s.Program] {
				return fmt.Errorf("mux %s has an invalid or duplicate program", m.Name)
			}
			programs[s.Program] = true
			if strings.ContainsAny(s.Name+s.Virtual, "\r\n\"\x00") {
				return errors.New("service name and virtual channel cannot contain quotes or control line breaks")
			}
		}
	}
	return nil
}

func ValidateBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("base URL must be an absolute http(s) URL without credentials, query or fragment")
	}
	return nil
}

func ReadJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("%s must contain one JSON document", path)
	}
	return nil
}

func Load(path string) (Config, error) {
	var c Config
	if err := ReadJSON(path, &c); err != nil {
		return c, err
	}
	return c, c.Validate()
}

func (c Config) Resolve(path string) Config {
	base, _ := filepath.Abs(filepath.Dir(path))
	if !filepath.IsAbs(c.StateDir) {
		c.StateDir = filepath.Join(base, c.StateDir)
	}
	if !filepath.IsAbs(c.RuntimeDir) {
		c.RuntimeDir = filepath.Join(base, c.RuntimeDir)
	}
	return c
}

func AtomicJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".ota-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err = f.Write(append(data, '\n')); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func CreateConfig(path string, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	return err
}

func ServicePath(m Mux, s Service) string {
	return fmt.Sprintf("/hls/%s/%d/index.m3u8", m.Name, s.Program)
}

func Playlist(c Config, base string) string {
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	for _, m := range c.Muxes {
		for _, s := range m.Services {
			fmt.Fprintf(&b, "#EXTINF:-1 tvg-id=\"%s-%d\" tvg-chno=\"%s\" group-title=\"OTA\",%s\n%s%s\n", m.Name, s.Program, s.Virtual, s.Name, strings.TrimRight(base, "/"), ServicePath(m, s))
		}
	}
	return b.String()
}
