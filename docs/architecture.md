# Architecture

OTA Headend is a local Go command-line application with separately installed
HDHomeRun and TSDuck tools. It has no dependency on the original monorepo or any
remote control plane.

```text
Antenna → HDHomeRun tuner (one physical RF multiplex)
                   ↓ native LAN capture
          hdhomerun_config save
                   ↓ full MPEG transport stream
          local process fanout
                   ├─ tsp zap PROGRAM A → HLS files ─┐
                   └─ tsp zap PROGRAM B → HLS files ─┤
                                                    ↓
                                             local HTTP origin
                                                    ↓
                                              HLS-capable player
```

`internal/devices/hdhomerun` and the capture/fanout implementation were extracted
from the newer internal worker implementation. The standalone configuration,
CLI, scanning workflow, ownership lifecycle, status/HTTP origin, documentation,
and installation/release packaging live entirely in this repository. No account,
enrollment, heartbeat, server-side grant, unrelated media integration, production
station lineup, or private endpoint is carried into the standalone application.

The capture implementation preserves per-service broken-pipe isolation. When an
extractor exits, that output is detached and the others continue. An extractor
that stays alive but stops reading can exert backpressure; this is not a fully
independent queued transport broker. Each service's recent playlist and actual
segment files determine readiness, not only whether its process exists.

The application checks tuner idleness, persists a private claim, then claims
the physical tuner using a random native lock key. Recovery authenticates the
saved key before clearing anything. Release clears target, filter, channel, and
lock in order; failed cleanup retains its saved record. A local advisory lock
prevents two CLI operations using the same state directory simultaneously.
These mechanisms cooperate with native HDHomeRun locking; they do not protect
against an administrator explicitly forcing a tuner reset through another tool.

The origin serves only configured playlists/segments, lineup, and health state.
There is no HTTP control API or authentication. Configuration is strict JSON;
external tools receive structured arguments rather than shell expressions.
The example systemd unit runs under an unprivileged account with durable and
transient directories explicitly managed by systemd.

This first standalone release deliberately does not port the old host-specific
fallback video pipeline, FFmpeg transforms, watchdog restarts, arbitrary guide
providers, recording, or remote orchestration. An unavailable program retains
its configured URL and reports unavailable; an explicit restart retries it.
No proprietary fallback video assets or personal configuration are included.
