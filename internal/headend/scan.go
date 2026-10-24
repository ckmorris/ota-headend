// Copyright 2026 Chris Morris.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
// See LICENSE.md.

package headend

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"ota-headend/internal/devices/hdhomerun"
)

type ScanResult struct {
	Version   int          `json:"version"`
	Device    string       `json:"device"`
	Tuner     int          `json:"tuner"`
	ScannedAt time.Time    `json:"scanned_at"`
	Muxes     []ScannedMux `json:"muxes"`
}
type ScannedMux struct {
	RF             int       `json:"rf"`
	FrequencyHz    int       `json:"frequency_hz"`
	SignalStrength int       `json:"signal_strength"`
	SignalQuality  int       `json:"signal_quality"`
	Services       []Service `json:"services"`
}

func ParseRFList(raw string) ([]int, error) {
	seen := map[int]bool{}
	for _, part := range strings.Split(raw, ",") {
		lo, hi, rangeOK := strings.Cut(strings.TrimSpace(part), "-")
		start, err := strconv.Atoi(lo)
		if err != nil {
			return nil, errors.New("use RF channels such as 2-36 or 7,11,19-22")
		}
		end := start
		if rangeOK {
			end, err = strconv.Atoi(hi)
			if err != nil {
				return nil, err
			}
		}
		if start < 2 || end > 36 || end < start {
			return nil, errors.New("RF channels must be between 2 and 36 in ascending ranges")
		}
		for n := start; n <= end; n++ {
			seen[n] = true
		}
	}
	var result []int
	for n := range seen {
		result = append(result, n)
	}
	sort.Ints(result)
	return result, nil
}

func Services(raw string) []Service {
	parsed := hdhomerun.ParseStreamInfo(raw)
	programs, _ := parsed["programs"].([]map[string]any)
	var services []Service
	for _, p := range programs {
		// Encrypted services cannot be extracted into playable clear HLS.
		name, _ := p["name"].(string)
		if strings.Contains(strings.ToLower(name), "encrypted") {
			continue
		}
		number, _ := p["program"].(int)
		virtual, _ := p["virtual_channel"].(string)
		if number < 1 || number > 65535 {
			continue
		}
		if name == "" {
			name = fmt.Sprintf("Program %d", number)
		}
		services = append(services, Service{Program: number, Virtual: virtual, Name: name})
	}
	return services
}

func (h Hardware) Scan(ctx context.Context, device string, tuner int, channels []int, progress io.Writer) (result ScanResult, err error) {
	result = ScanResult{Version: 1, Device: device, Tuner: tuner, ScannedAt: time.Now().UTC(), Muxes: []ScannedMux{}}
	claim, err := h.Acquire(ctx, device, tuner)
	if err != nil {
		return result, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err = errors.Join(err, h.Release(cleanup, claim))
	}()
	for _, rf := range channels {
		freq, e := Frequency(rf)
		if e != nil {
			return result, e
		}
		fmt.Fprintf(progress, "Scanning RF %d...\n", rf)
		if err := h.set(ctx, claim, "channel", fmt.Sprintf("8vsb:%d", freq)); err != nil {
			return result, err
		}
		var services []Service
		var status map[string]any
		// Allow the RF lock and program table to settle; a lock alone is not a lineup.
		for attempt := 0; attempt < 5; attempt++ {
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			case <-time.After(500 * time.Millisecond):
			}
			raw, e := h.get(ctx, device, tuner, "status")
			if e != nil {
				return result, e
			}
			status = hdhomerun.ParseTunerStatus(device, tuner, raw)
			if status["lock"] != "8vsb" {
				continue
			}
			info, e := h.get(ctx, device, tuner, "streaminfo")
			if e != nil {
				return result, e
			}
			services = Services(info)
			if len(services) > 0 {
				break
			}
		}
		if len(services) == 0 {
			continue
		}
		strength, _ := status["signal_strength"].(int)
		quality, _ := status["signal_quality"].(int)
		result.Muxes = append(result.Muxes, ScannedMux{RF: rf, FrequencyHz: freq, SignalStrength: strength, SignalQuality: quality, Services: services})
		for _, s := range services {
			fmt.Fprintf(progress, "  RF %d / program %d / %s %s\n", rf, s.Program, s.Virtual, s.Name)
		}
	}
	return result, nil
}
