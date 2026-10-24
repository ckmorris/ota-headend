// Copyright 2026 Chris Morris.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
// See LICENSE.md.

package headend

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestStartupReadyWaitsForAllServicesAndOnlyAnnouncesOnce(t *testing.T) {
	var out bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&out, nil))
	r := &startupReadiness{}
	r.update(logger, Status{Running: true})
	status := Status{Running: true, Outputs: []OutputStatus{{Mux: "first", Ready: true}, {Mux: "second", Ready: false}}}
	r.update(logger, status)
	if out.Len() != 0 {
		t.Fatal("announced readiness before all services were ready:", out.String())
	}
	status.Outputs[1].Ready = true
	r.update(logger, status)
	r.update(logger, status)
	status.Outputs[0].Ready = false
	r.update(logger, status)
	status.Outputs[0].Ready = true
	r.update(logger, status)
	if strings.Count(out.String(), "Ready!") != 1 || !strings.Contains(out.String(), "services=2") {
		t.Fatal("expected one readiness announcement:", out.String())
	}
}
