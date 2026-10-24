// Copyright 2026 Chris Morris.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
// See LICENSE.md.

package headend

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"ota-headend/internal/devices/hdhomerun"
)

type Runner interface {
	Run(context.Context, string, ...string) (string, error)
}
type Hardware struct {
	Runner   Runner
	StateDir string
}
type Claim struct {
	Device string `json:"device"`
	Tuner  int    `json:"tuner"`
	Key    string `json:"key"`
}

func (h Hardware) get(ctx context.Context, device string, tuner int, field string) (string, error) {
	return h.Runner.Run(ctx, "hdhomerun_config", device, "get", fmt.Sprintf("/tuner%d/%s", tuner, field))
}
func (h Hardware) set(ctx context.Context, c Claim, field, value string) error {
	_, err := h.Runner.Run(ctx, "hdhomerun_config", c.Device, "key", c.Key, "set", fmt.Sprintf("/tuner%d/%s", c.Tuner, field), value)
	// Do not include the command line or key in returned errors/logs.
	if err != nil {
		return fmt.Errorf("%s tuner%d: could not set %s", c.Device, c.Tuner, field)
	}
	return nil
}
func (h Hardware) claimPath(c Claim) string {
	return filepath.Join(h.StateDir, "claims", fmt.Sprintf("%s-tuner%d.json", c.Device, c.Tuner))
}

func (h Hardware) Acquire(ctx context.Context, device string, tuner int) (Claim, error) {
	c := Claim{Device: device, Tuner: tuner}
	if err := ValidDevice(device, tuner); err != nil {
		return c, err
	}
	for _, field := range []string{"lockkey", "target"} {
		out, err := h.get(ctx, device, tuner, field)
		if err != nil || strings.TrimSpace(out) != "none" {
			return c, fmt.Errorf("%s tuner%d is busy or unreadable (%s); choose an idle tuner", device, tuner, field)
		}
	}
	raw, err := h.get(ctx, device, tuner, "status")
	if err != nil {
		return c, err
	}
	status := hdhomerun.ParseTunerStatus(device, tuner, raw)
	if status["channel"] != "none" || status["lock"] != "none" || status["bps"] != 0 || status["pps"] != 0 {
		return c, fmt.Errorf("%s tuner%d is tuned or status is incomplete; choose an idle tuner", device, tuner)
	}
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return c, err
	}
	n := binary.BigEndian.Uint32(b[:])
	if n == 0 {
		n = 1
	}
	c.Key = strconv.FormatUint(uint64(n), 10)
	// Never replace an existing claim, including one from a failed cleanup.
	if _, err := os.Stat(h.claimPath(c)); !errors.Is(err, os.ErrNotExist) {
		return Claim{}, errors.New("saved ownership exists; run recover before claiming this tuner")
	}
	if err := AtomicJSON(h.claimPath(c), c); err != nil {
		return Claim{}, err
	}
	if err := h.set(ctx, c, "lockkey", c.Key); err != nil {
		return Claim{}, err
	}
	return c, nil
}

func (h Hardware) Release(ctx context.Context, c Claim) error {
	for _, field := range []string{"target", "filter", "channel", "lockkey"} {
		if err := h.set(ctx, c, field, "none"); err != nil {
			return fmt.Errorf("%w; saved ownership retained for recover", err)
		}
	}
	return os.Remove(h.claimPath(c))
}

// Recovery clears only authenticated saved claims. An unlocked/reset tuner is
// left untouched and will need fresh idle checks before any new acquisition.
func (h Hardware) Recover(ctx context.Context) error {
	entries, err := os.ReadDir(filepath.Join(h.StateDir, "claims"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		var c Claim
		path := filepath.Join(h.StateDir, "claims", e.Name())
		if err := ReadJSON(path, &c); err != nil {
			return fmt.Errorf("invalid ownership record %s; refusing recovery", e.Name())
		}
		key, err := strconv.ParseUint(c.Key, 10, 32)
		if err != nil || key == 0 || ValidDevice(c.Device, c.Tuner) != nil || h.claimPath(c) != path {
			return errors.New("saved ownership does not match its device/tuner; refusing recovery")
		}
		owner, err := h.get(ctx, c.Device, c.Tuner, "lockkey")
		if err != nil {
			return errors.New("cannot verify saved tuner ownership; retain state and retry recover")
		}
		if strings.TrimSpace(owner) == "none" {
			if err := os.Remove(path); err != nil {
				return err
			}
			continue
		}
		if strings.TrimSpace(owner) == "" {
			return errors.New("empty lock response; refusing recovery")
		}
		if err := h.set(ctx, c, "lockkey", c.Key); err != nil {
			return fmt.Errorf("%s tuner%d rejected saved ownership; refusing takeover", c.Device, c.Tuner)
		}
		if err := h.Release(ctx, c); err != nil {
			return err
		}
	}
	return nil
}
