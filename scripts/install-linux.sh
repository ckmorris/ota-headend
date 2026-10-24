#!/usr/bin/env bash
# Copyright 2026 Chris Morris. See LICENSE.md.
# SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
project_dir="$(dirname -- "$script_dir")"
binary="$project_dir/bin/ota-headend"
config=""
accepted=false
while (($#)); do
  case "$1" in
    --config) config="${2:?--config needs a path}"; shift 2 ;;
    --binary) binary="${2:?--binary needs a path}"; shift 2 ;;
    --accept-license) accepted=true; shift ;;
    --help|-h)
      echo 'Usage: sudo ./scripts/install-linux.sh --config FILE [--binary FILE] --accept-license'
      echo '--accept-license confirms agreement to LICENSE.md and EULA.md, including legitimate use only and no piracy.'
      exit 0 ;;
    *) echo "Unknown argument: $1" >&2; exit 1 ;;
  esac
done
if [[ "$accepted" != true ]]; then
  echo "Read $project_dir/LICENSE.md and $project_dir/EULA.md. Re-run with --accept-license to agree to the license and end-user terms, including legitimate use only and no piracy." >&2
  exit 1
fi
echo 'Terms accepted: LICENSE.md and EULA.md, including legitimate use only and no piracy.'
[[ "$(uname -s)" == Linux ]] || { echo 'This installer requires Linux.' >&2; exit 1; }
[[ "$EUID" -eq 0 ]] || { echo 'Run the installer with sudo.' >&2; exit 1; }
[[ -f "$config" && -x "$binary" ]] || { echo 'Provide --config FILE and a built binary (make build).' >&2; exit 1; }
for tool in python3 systemctl useradd getent hdhomerun_config tsp; do
  command -v "$tool" >/dev/null || { echo "Missing dependency: $tool" >&2; exit 1; }
done
"$binary" validate --config "$config"
if systemctl is-active --quiet ota-headend; then
  echo 'Stop ota-headend before installing or upgrading.' >&2
  exit 1
fi
if ! getent passwd ota-headend >/dev/null; then
  useradd --system --user-group --home-dir /var/lib/ota-headend --shell /usr/sbin/nologin ota-headend
fi
install -d -o root -g ota-headend -m 0750 /etc/ota-headend
install -d -o ota-headend -g ota-headend -m 0700 /var/lib/ota-headend
install -d -o ota-headend -g ota-headend -m 0750 /run/ota-headend
if [[ ! -e /etc/ota-headend/config.json ]]; then
  # Normalize service paths rather than resolving foreground state under /etc.
  python3 - "$config" /etc/ota-headend/config.json <<'PY'
import json, os, sys
with open(sys.argv[1], encoding="utf-8") as source:
    config = json.load(source)
config["state_dir"] = "/var/lib/ota-headend"
config["runtime_dir"] = "/run/ota-headend"
fd = os.open(sys.argv[2], os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o640)
with os.fdopen(fd, "w", encoding="utf-8") as target:
    json.dump(config, target, indent=2)
    target.write("\n")
PY
  chown ota-headend:ota-headend /etc/ota-headend/config.json
else
  echo 'Preserving existing /etc/ota-headend/config.json and durable tuner ownership state.'
fi
"$binary" validate --config /etc/ota-headend/config.json
# The offline CLI saves config atomically, requiring directory write permission.
# ProtectSystem=strict still makes it read-only within the running systemd unit.
chown ota-headend:ota-headend /etc/ota-headend
install -m 0755 "$binary" /usr/local/bin/ota-headend
install -m 0644 "$project_dir/packaging/systemd/ota-headend.service" /etc/systemd/system/ota-headend.service
install -d -m 0755 /usr/local/share/doc/ota-headend
for document in LICENSE.md EULA.md CLA.md THIRD_PARTY_NOTICES.md README.md; do
  install -m 0644 "$project_dir/$document" /usr/local/share/doc/ota-headend/
done
systemctl daemon-reload
echo 'Installed; no capture started. Start with: sudo systemctl enable --now ota-headend'
