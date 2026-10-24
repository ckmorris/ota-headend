// Copyright 2026 Chris Morris.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
// See LICENSE.md.

package capture

import (
	"bytes"
	"strings"
	"testing"
)

func TestSaveDiagnosticsSuppressesHeartbeatAndPreservesErrors(t *testing.T) {
	input := "....n..o.t.s...\nunable to connect to 192.168.1.20.\nnot enough data...\n..."
	want := "HDHomeRun capture: network error reported\n" +
		"HDHomeRun capture: overflow error reported\n" +
		"HDHomeRun capture: transport error reported\n" +
		"HDHomeRun capture: sequence error reported\n" +
		"unable to connect to 192.168.1.20.\nnot enough data...\n"
	// Pipes may deliver the same output in any-sized chunks.
	for size := 1; size <= len(input); size++ {
		var out bytes.Buffer
		w := &saveDiagnostics{output: &out}
		for offset := 0; offset < len(input); offset += size {
			chunk := input[offset:min(offset+size, len(input))]
			if n, err := w.Write([]byte(chunk)); err != nil || n != len(chunk) {
				t.Fatal(n, err)
			}
		}
		if err := w.Flush(); err != nil {
			t.Fatal(err)
		}
		if out.String() != want {
			t.Fatalf("chunk size %d: got %q, want %q", size, out.String(), want)
		}
	}
	var out bytes.Buffer
	w := &saveDiagnostics{output: &out}
	_, _ = w.Write([]byte(strings.Repeat(".", 10000)))
	_, _ = w.Write([]byte("n"))
	_ = w.Flush()
	if out.String() != "HDHomeRun capture: network error reported\n" {
		t.Fatalf("heartbeat or final error mishandled: %q", out.String())
	}
}
