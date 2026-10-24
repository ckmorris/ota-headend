#!/usr/bin/env bash
# Copyright 2026 Chris Morris. See LICENSE.md.
# SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."
unformatted="$(gofmt -l cmd internal)"
[[ -z "$unformatted" ]] || { echo "Run gofmt on: $unformatted" >&2; exit 1; }
go vet ./...
go test -race ./...
for script in scripts/*.sh; do bash -n "$script"; done
for config in config/headend.json examples/*.json; do
  go run ./cmd/ota-headend validate --config "$config"
done
echo 'Code, race tests, shell syntax, and example configuration checks passed.'
