// Copyright 2026 Chris Morris.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
// See LICENSE.md.

package capture

import (
	"errors"
	"io"
	"sync"
)

// A failed service must not close transport delivery to the other services.
type transportFanout struct {
	mu      sync.Mutex
	outputs []io.WriteCloser
	paths   []string
	failed  map[string]string
}

func (f *transportFanout) Write(data []byte) (int, error) {
	live := 0
	for i, output := range f.outputs {
		if output == nil {
			continue
		}
		n, err := output.Write(data)
		if err != nil || n != len(data) {
			_ = output.Close()
			f.outputs[i] = nil
			f.mu.Lock()
			if f.failed == nil {
				f.failed = map[string]string{}
			}
			f.failed[f.paths[i]] = "Service extraction stopped; other outputs are unaffected."
			f.mu.Unlock()
			continue
		}
		live++
	}
	if live == 0 {
		return 0, errors.New("all managed OTA service outputs stopped")
	}
	return len(data), nil
}

func (f *transportFanout) failures() map[string]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make(map[string]string, len(f.failed))
	for path, message := range f.failed {
		result[path] = message
	}
	return result
}
