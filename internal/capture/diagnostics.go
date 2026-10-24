// Copyright 2026 Chris Morris.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
// See LICENSE.md.

package capture

import "io"

// hdhomerun_config save emits a dot per second on stderr, or o/n/t/s for
// capture errors. Filter these independently of pipe read boundaries while
// preserving ordinary diagnostic text, including punctuation inside messages.
type saveDiagnostics struct {
	output  io.Writer
	pending []byte
	inLine  bool
}

func (w *saveDiagnostics) progress() []byte {
	var data []byte
	for _, marker := range w.pending {
		var message string
		switch marker {
		case 'o':
			message = "overflow"
		case 'n':
			message = "network"
		case 't':
			message = "transport"
		case 's':
			message = "sequence"
		}
		data = append(data, "HDHomeRun capture: "+message+" error reported\n"...)
	}
	w.pending = nil
	return data
}

func (w *saveDiagnostics) Write(p []byte) (int, error) {
	var data []byte
	for _, b := range p {
		if w.inLine {
			data = append(data, b)
			w.inLine = b != '\n' && b != '\r'
			continue
		}
		switch b {
		case '.', '\n', '\r':
			data = append(data, w.progress()...)
		case 'o', 'n', 't', 's':
			w.pending = append(w.pending, b)
			if len(w.pending) >= 64 {
				data = append(data, w.progress()...)
			}
		default:
			data = append(data, w.pending...)
			w.pending = nil
			data = append(data, b)
			w.inLine = true
		}
	}
	if len(data) > 0 {
		if _, err := w.output.Write(data); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func (w *saveDiagnostics) Flush() error {
	if len(w.pending) == 0 {
		return nil
	}
	_, err := w.output.Write(w.progress())
	return err
}
