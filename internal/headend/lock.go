// Copyright 2026 Chris Morris.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
// See LICENSE.md.

package headend

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// An advisory process lock supplements the device lock, including during scans.
func Lock(stateDir string) (func(), error) {
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(stateDir, "headend.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("another command is using %s; stop the running headend before scanning or editing configuration", stateDir)
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
}
