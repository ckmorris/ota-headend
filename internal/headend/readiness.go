// Copyright 2026 Chris Morris.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
// See LICENSE.md.

package headend

import "log/slog"

type startupReadiness struct{ announced bool }

func (r *startupReadiness) update(logger *slog.Logger, status Status) {
	if r.announced || !status.Running || len(status.Outputs) == 0 {
		return
	}
	for _, output := range status.Outputs {
		if !output.Ready {
			return
		}
	}
	logger.Info("Ready! All configured services are streaming.", "services", len(status.Outputs))
	r.announced = true
}
