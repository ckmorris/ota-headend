// Copyright 2026 Chris Morris.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
// See LICENSE.md.

package capture

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// Re-execute this test binary as fake transport/TSDuck children, never hardware tools.
func TestMain(m *testing.M) {
	if os.Getenv("OTA_HEADEND_TEST_CAPTURE_CHILD") == "1" {
		switch filepath.Base(os.Args[0]) {
		case "hdhomerun_config":
			for range 10_000 {
				if _, err := os.Stdout.Write(make([]byte, 188*64)); err != nil {
					os.Exit(0)
				}
				time.Sleep(2 * time.Millisecond)
			}
			os.Exit(0)
		case "tsp":
			if slices.Contains(os.Args, "8") {
				os.Exit(1)
			}
			_, _ = io.Copy(io.Discard, os.Stdin)
			os.Exit(0)
		}
		os.Exit(2)
	}
	os.Exit(m.Run())
}

func fakeCaptureTools(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"hdhomerun_config", "tsp"} {
		if err := os.Symlink(executable, filepath.Join(bin, name)); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	t.Setenv("OTA_HEADEND_TEST_CAPTURE_CHILD", "1")
}

func TestPipelineSurvivesOneExtractorExitAndStopsCleanly(t *testing.T) {
	fakeCaptureTools(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	process, err := (ExecCaptureRunner{}).Start(ctx, CaptureRequest{DeviceID: "fake", Outputs: []CaptureOutput{
		{ProgramNumber: 5, PlaylistPath: "present"},
		{ProgramNumber: 8, PlaylistPath: "absent"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	p := process.(*pipelineProcess)
	deadline := time.After(5 * time.Second)
	for len(p.OutputFailures()) != 1 {
		select {
		case err := <-p.Done():
			t.Fatalf("one missing service terminated the entire pipeline: %v", err)
		case <-deadline:
			t.Fatal("extractor failure was not isolated")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	select {
	case <-p.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("pipeline did not reap children after cancellation")
	}
}

func TestPipelineAllExtractorsExitDoesNotDeadlockProducerWait(t *testing.T) {
	fakeCaptureTools(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	process, err := (ExecCaptureRunner{}).Start(ctx, CaptureRequest{DeviceID: "fake", Outputs: []CaptureOutput{{ProgramNumber: 8, PlaylistPath: "absent"}}})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-process.Done():
		if err == nil {
			t.Fatal("all failed services reported success")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("producer wait deadlocked after all extractors exited")
	}
}
