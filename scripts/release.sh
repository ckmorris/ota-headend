#!/usr/bin/env bash
# Copyright 2026 Chris Morris. See LICENSE.md.
# SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
# Official releases are published by Chris Morris or an authorized operator.
set -euo pipefail
# Keep macOS resource-fork metadata out of Linux and source archives.
export COPYFILE_DISABLE=1
cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."
release_version="${1:-}"
if [[ ! "$release_version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-z0-9.]+)?$ ]]; then
  echo 'Usage: ./scripts/release.sh v0.1.0[-rc.1]' >&2
  exit 1
fi
./scripts/check.sh
release_dir="dist/$release_version"
[[ ! -e "$release_dir" ]] || { echo "Refusing to overwrite $release_dir" >&2; exit 1; }
mkdir -p "$release_dir"
for arch in amd64 arm64; do
  name="ota-headend-${release_version}-linux-${arch}"
  stage="$release_dir/$name"
  mkdir -p "$stage/bin" "$stage/packaging" "$stage/config"
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -ldflags "-s -w -X main.version=$release_version" -o "$stage/bin/ota-headend" ./cmd/ota-headend
  cp LICENSE.md EULA.md CLA.md THIRD_PARTY_NOTICES.md README.md CONTRIBUTING.md SECURITY.md CHANGELOG.md "$stage/"
  cp -R docs examples scripts "$stage/"
  # Generate a clean starter instead of copying a locally populated config.
  cp config/.keep config/README.md "$stage/config/"
  go run ./cmd/ota-headend init --config "$stage/config/headend.json"
  cp -R packaging/systemd "$stage/packaging/"
  # Preserve the exact toolchain notices, including bundled standard-library code.
  go_root="$(go env GOROOT)"
  go_license="$go_root/LICENSE"
  if [[ ! -f "$go_license" ]]; then go_license="$(dirname "$go_root")/LICENSE"; fi
  [[ -f "$go_license" ]] || { echo 'Cannot find the Go toolchain license.' >&2; exit 1; }
  cp "$go_license" "$stage/GO-LICENSE.txt"
  if [[ -f "$go_root/PATENTS" ]]; then cp "$go_root/PATENTS" "$stage/GO-PATENTS.txt"; fi
  if [[ -d "$go_root/src/vendor" ]]; then
    mkdir -p "$stage/GO-VENDOR-NOTICES"
    while IFS= read -r notice; do
      relative="${notice#"$go_root/src/vendor/"}"
      mkdir -p "$stage/GO-VENDOR-NOTICES/$(dirname "$relative")"
      cp "$notice" "$stage/GO-VENDOR-NOTICES/$relative"
    done < <(find "$go_root/src/vendor" -type f \( -name LICENSE -o -name 'LICENSE.*' -o -name PATENTS -o -name NOTICE -o -name 'NOTICE.*' \))
  fi
  go version -m "$stage/bin/ota-headend" > "$stage/BUILD-INFO.txt"
  tar -czf "$release_dir/$name.tar.gz" -C "$release_dir" "$name"
done
# Explicit allowlist: never package local config, scan results, claims or media.
tar -czf "$release_dir/ota-headend-${release_version}-source.tar.gz" \
  --exclude='*.DS_Store' \
  go.mod Makefile .gitignore LICENSE.md EULA.md CLA.md THIRD_PARTY_NOTICES.md README.md CONTRIBUTING.md SECURITY.md CHANGELOG.md \
  cmd internal docs examples scripts packaging config/.keep config/README.md \
  -C "$release_dir/ota-headend-${release_version}-linux-amd64" config/headend.json
python3 - "$release_dir" <<'PY'
import hashlib, pathlib, sys
root = pathlib.Path(sys.argv[1])
with (root / "SHA256SUMS").open("w", encoding="utf-8") as output:
    for archive in sorted(root.glob("*.tar.gz")):
        output.write(f"{hashlib.sha256(archive.read_bytes()).hexdigest()}  {archive.name}\n")
PY
echo "Archives and SHA256SUMS created in $release_dir. Nothing uploaded or published."
