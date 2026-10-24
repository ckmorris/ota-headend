# Validation and releases

Chris Morris or an authorized operator publishes official project releases.
Others may distribute copies and modified versions under LICENSE.md; those
distributions do not imply maintainer endorsement. The release script creates
local archives; it does not upload, create a remote repository, or publish
anything.

## Automated checks

Run `make check`. This verifies formatting, static analysis, race-enabled Go
tests, shell syntax, and the example configuration. Tests cover discovery
parsing and read-only probes, process fanout with real fake child executables,
broken extractor isolation, ownership recovery/refusal/cleanup, configuration
round trips, HTTP file restrictions, and readiness. Synthetic bytes in tests are
not a decodable broadcast media sample.

Cross-compilation confirms the application builds for Linux; it does not test
native TSDuck packages, systemd execution, RF reception, LAN UDP capture, actual
codec playback, or load. Those need the target hardware.

## Before the first public release

- Publish the official download, commercial licensing contact, private security
  contact, and contribution submission mechanism in the project repository on
  the [ckmorris GitHub account](https://github.com/ckmorris).
  Update documentation if you choose permanent direct contact links.
- Confirm Chris Morris owns or can license all extracted code and assets under
  PolyForm Noncommercial 1.0.0. Preserve required notices and retain contributor
  agreement acceptance records for submitted work.
- On a clean reference Linux host, follow the README from downloaded archive to
  first playback without access to the original development repository.
- Verify no-tools, missing-config, no-device, busy-tuner, and interrupted-scan
  cases. Confirm a DVR's occupied tuner is not changed.
- Scan and record expected local RF/program mappings. Test at least one low-VHF
  frequency if available, several services on one mux, and a second physical tuner.
- Play the served lineup and individual URLs in a compatible player. Confirm
  steady playback and reasonable storage/memory for your actual bitrate.
- Remove signal, observe stale output become HTTP 503, restore signal, and check
  the documented explicit restart behavior.
- Test clean Ctrl-C, systemd stop/start, host reboot, abrupt process termination,
  authenticated saved-claim recovery, and unknown-lock refusal.
- Exercise `install-linux.sh` on first install and upgrade; verify account
  permissions, tool PATH, config preservation, state retention, journald, and boot.
- Complete a sustained soak on the exact release build and dependency versions.
  Record host, architecture, tuner model, firmware, versions, duration, and results.
- Verify both architectures on real target systems before describing both as
  hardware-tested. Keep cross-build results distinct from runtime acceptance.

## Build a candidate

For a single Linux amd64 deployment binary in `bin/ota-headend`, run
`make build-linux-amd64`. This cross-compiles on a Mac as well as on Linux.
For local execution on the build host, use `make build` instead. See
[Go installation and build targets](install-go.md).

```sh
make release VERSION=v0.1.0-rc.1
```

This runs checks and creates Linux amd64 and arm64 archives, an allowlisted
source archive, and `SHA256SUMS` under `dist/v0.1.0-rc.1/`. Binary archives include
the license, end-user terms, CLA, third-party/toolchain notices, build metadata,
installer, service, example configuration, and user documentation. The script
refuses to overwrite an existing version directory. Give a corrected candidate
a new version; do not replace published bytes under the same checksum/version.

Inspect archive file lists before upload. No local configuration, scan results,
tuner claims, logs, binary build caches, or HLS media should be present. Verify
the checksums independently:

```sh
cd dist/v0.1.0-rc.1
sha256sum -c SHA256SUMS
```

On macOS use `shasum -a 256 -c SHA256SUMS`. Transfer approved archives/checksums
to the official GitHub repository using your normal release process. Make the
license and commercial licensing contact visible on the release page. Keep
reviewed source corresponding to each released binary and preserve CLA acceptance
records for contributions. `scripts/check.sh` can be invoked by any CI system;
the project has no dependency on a specific code host.

## Upgrade and rollback

Stop the service cleanly, back up `/etc/ota-headend/config.json` and
`/var/lib/ota-headend`, then run the new installer with license acceptance. It
preserves installed configuration and claims. Start the service and test status
and playback. A rollback follows the same stop/install/start sequence using
the previous binary; retain compatible configuration and durable ownership state.
Do not restore a stale claim snapshot over currently running hardware ownership.
