// Copyright 2026 Chris Morris.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
// See LICENSE.md.

package devices

import "context"

type Observation struct {
	Kind               string         `json:"kind"`
	HardwareID         string         `json:"hardware_id"`
	Manufacturer       string         `json:"manufacturer,omitempty"`
	Model              string         `json:"model,omitempty"`
	EndpointCandidates []string       `json:"endpoint_candidates"`
	Capabilities       map[string]any `json:"capabilities,omitempty"`
	State              map[string]any `json:"state,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
}

type Discoverer interface {
	Discover(ctx context.Context) ([]Observation, error)
}
