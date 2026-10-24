# Changelog

## 0.1.0 — Unreleased

- Standalone OTA Headend, attributed to Chris Morris.
- Local discovery, US ATSC 1.0 RF scanning, scan-based configuration, and M3U
  lineup export.
- One HDHomeRun capture per multiplex and separate native-codec TSDuck HLS
  output per selected program.
- Private persisted tuner ownership, crash recovery, and refusal of busy tuners.
- Per-service readiness, bounded HLS file serving, and local status snapshots.
- Optional unprivileged Linux systemd service; source and Linux amd64/arm64
  release packaging with checksums.
- LAN playback by default at `0.0.0.0:8090`, with detected IPs and full IPTV
  playlist URLs in startup logs and status. Served M3U channel URLs use the
  connection's local address unless a player-facing base URL is configured.
- One combined M3U across all configured multiplexes and devices; macOS/Homebrew
  and Ubuntu/systemd setup guides.
- Quiet capture output with a one-time `Ready!` announcement when all services
  are playable; HDHomeRun capture errors remain visible.
- PolyForm Noncommercial 1.0.0 source-available license and contributor
  agreement, with Chris Morris as licensor. Noncommercial modifications and
  redistribution are permitted; commercial use outside the license's permitted
  purposes requires a separate paid written license.

Real-hardware acceptance and publication are pending. See docs/releasing.md.
