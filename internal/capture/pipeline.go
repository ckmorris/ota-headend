// Copyright 2026 Chris Morris.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
// See LICENSE.md.

package capture

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

type CaptureRunner interface {
	Start(ctx context.Context, request CaptureRequest) (Process, error)
}

type CaptureRequest struct {
	DeviceID   string
	TunerIndex int
	LockKey    string
	Outputs    []CaptureOutput
}

type CaptureOutput struct {
	ProgramNumber   int
	PlaylistPath    string
	SegmentTemplate string
}

type Process interface {
	Stop(context.Context) error
	Done() <-chan error
}

type ExecCaptureRunner struct{}

func (ExecCaptureRunner) Start(ctx context.Context, request CaptureRequest) (Process, error) {
	saveArgs := []string{request.DeviceID}
	if request.LockKey != "" {
		saveArgs = append(saveArgs, "key", request.LockKey)
	}
	saveArgs = append(saveArgs, "save", fmt.Sprintf("/tuner%d", request.TunerIndex), "-")
	saveCmd := exec.CommandContext(ctx, "hdhomerun_config", saveArgs...)
	if len(request.Outputs) == 0 {
		return nil, errors.New("capture request has no outputs")
	}
	stdout, err := saveCmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	tail := &safeTail{}
	diagnostics := &saveDiagnostics{output: io.MultiWriter(os.Stderr, tail)}
	saveCmd.Stderr = diagnostics
	tspCmds := make([]*exec.Cmd, 0, len(request.Outputs))
	tspInputs := make([]io.WriteCloser, 0, len(request.Outputs))
	for _, output := range request.Outputs {
		args := []string{"--realtime", "-I", "file", "-"}
		if output.ProgramNumber > 0 {
			args = append(args, "-P", "zap", strconv.Itoa(output.ProgramNumber))
		}
		args = append(args,
			"-O", "hls",
			"--align-first-segment", "--intra-close",
			"--playlist", output.PlaylistPath,
			"--live", "6",
			"--duration", "4",
			output.SegmentTemplate,
		)
		tspCmd := exec.CommandContext(ctx, "tsp", args...)
		input, err := tspCmd.StdinPipe()
		if err != nil {
			return nil, err
		}
		tspCmd.Stderr = io.MultiWriter(os.Stderr, tail)
		tspCmds = append(tspCmds, tspCmd)
		tspInputs = append(tspInputs, input)
	}
	if err := saveCmd.Start(); err != nil {
		return nil, err
	}
	for _, tspCmd := range tspCmds {
		if err := tspCmd.Start(); err != nil {
			_ = saveCmd.Process.Kill()
			for _, started := range tspCmds {
				if started.Process != nil {
					_ = started.Process.Kill()
				}
			}
			_ = stdout.Close()
			for _, input := range tspInputs {
				_ = input.Close()
			}
			_ = saveCmd.Wait()
			for _, started := range tspCmds {
				if started.Process != nil {
					_ = started.Wait()
				}
			}
			return nil, err
		}
	}
	done := make(chan error, 1)
	paths := make([]string, len(request.Outputs))
	for i, output := range request.Outputs {
		paths[i] = output.PlaylistPath
	}
	fanout := &transportFanout{outputs: slices.Clone(tspInputs), paths: paths}
	tspResults := make(chan error, len(tspCmds))
	for _, cmd := range tspCmds {
		go func() { tspResults <- cmd.Wait() }()
	}
	go func() {
		_, copyErr := io.Copy(fanout, stdout)
		for _, input := range tspInputs {
			_ = input.Close()
		}
		if copyErr != nil {
			_ = saveCmd.Process.Kill()
			for _, cmd := range tspCmds {
				_ = cmd.Process.Kill()
			}
		}
		saveErr := saveCmd.Wait()
		_ = diagnostics.Flush()
		var tspErr error
		for range tspCmds {
			if err := <-tspResults; err != nil && tspErr == nil {
				tspErr = err
			}
		}
		if copyErr != nil {
			done <- fmt.Errorf("managed OTA capture copy failed: %w: %s", copyErr, tail.String())
			return
		}
		if saveErr != nil {
			done <- fmt.Errorf("hdhomerun_config save exited: %w: %s", saveErr, tail.String())
			return
		}
		if tspErr != nil {
			done <- fmt.Errorf("tsp exited: %w: %s", tspErr, tail.String())
			return
		}
		done <- nil
	}()
	processes := []*os.Process{saveCmd.Process}
	for _, tspCmd := range tspCmds {
		processes = append(processes, tspCmd.Process)
	}
	return &pipelineProcess{processes: processes, done: done, fanout: fanout}, nil
}

type pipelineProcess struct {
	processes []*os.Process
	done      <-chan error
	fanout    *transportFanout
}

func (p *pipelineProcess) OutputFailures() map[string]string { return p.fanout.failures() }

func (p *pipelineProcess) Stop(ctx context.Context) error {
	for _, process := range p.processes {
		if process != nil {
			_ = process.Signal(os.Interrupt)
		}
	}
	select {
	case err := <-p.done:
		return err
	case <-ctx.Done():
		for _, process := range p.processes {
			if process != nil {
				_ = process.Kill()
			}
		}
		select {
		case <-p.done:
		case <-time.After(2 * time.Second):
		}
		return ctx.Err()
	case <-time.After(3 * time.Second):
		for _, process := range p.processes {
			if process != nil {
				_ = process.Kill()
			}
		}
		select {
		case <-p.done:
		case <-time.After(2 * time.Second):
		}
		return errors.New("capture pipeline stop timed out")
	}
}

func (p *pipelineProcess) Done() <-chan error {
	return p.done
}

type safeTail struct {
	mu sync.Mutex
	b  []byte
}

func (t *safeTail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.b = append(t.b, p...)
	if len(t.b) > 4096 {
		t.b = t.b[len(t.b)-4096:]
	}
	return len(p), nil
}

func (t *safeTail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.TrimSpace(string(t.b))
}
