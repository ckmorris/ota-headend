# Commands and configuration

Every command accepts `--config PATH` after the command name. The guides use
`--config config/headend.json`, the valid empty starter included in the
[config directory](../config/README.md). The CLI's fallback when `--config` is
omitted remains `./config.json`; pass the documented path for foreground use.
`discover` does not read configuration. Use `COMMAND --help`
for the actual supported options. The application never executes a shell command
received through configuration or HTTP.

| Command | Hardware changes | Purpose |
| --- | --- | --- |
| `init` | None | Create a new configuration |
| `doctor` | None | Check PATH and config |
| `discover [--json]` | None | Read devices and tuner observations |
| `scan --device ID --tuner N [--rf LIST] [--output FILE]` | Temporarily tunes selected idle tuner | Save fresh scan results |
| `add --scan FILE --rf N --name NAME [--device ID] [--tuner N] [--programs LIST]` | None | Add a multiplex from scan results |
| `remove --name NAME` | None | Remove a multiplex while stopped |
| `validate` | None | Strictly validate JSON and assignments |
| `run --accept-terms` | Tunes configured idle tuners | Accept end-user terms, then capture and serve HLS until signaled |
| `status [--json]` | None | Read local status; nonzero exit if unhealthy |
| `playlist [--base-url URL]` | None | Write an M3U lineup to stdout |
| `recover` | Releases authenticated saved claims only | Recover after a crash |

`scan`, `add`, `remove`, `recover`, and `run` acquire an exclusive lock in the
resolved state directory. Stop the active process before running them. Use one
state directory per installation; do not run parallel configurations against the
same physical tuners. `add` and `remove` write config atomically and start no
capture. Configuration is read at startup, without live reload.

Before starting a release with these terms, read [LICENSE.md](../LICENSE.md) and
[EULA.md](../EULA.md). `run` requires `--accept-terms` to confirm your agreement
to legitimate use only and no piracy. The command rejects missing acceptance
before reading configuration, acquiring locks, or interacting with tuners.

All `muxes` entries in one configuration are launched by one `run` command.
Every configured service appears in the same `/lineup.m3u`, across all devices
and tuners. Repeat `add --config SAME_FILE ...` for each multiplex; omitting
`--programs` includes all clear services on the selected scanned RF channel.
Multiple scan-result files in `config/` can feed the same `config/headend.json`.

## File format

See [examples/config.json](../examples/config.json) for an illustrative, populated
configuration. `init` produces an empty lineup so that no example device is
accidentally tuned.

For two four-tuner SiliconDust HDHomeRun devices using seven of eight tuners,
see [the Atlanta example](../examples/config.atlanta-7-tuners.json) and its
[tuner assignment guide](../examples/README.md). Replace the placeholder device
IDs and confirm station mappings with a scan before running it.

```json
{
  "version": 1,
  "listen": "0.0.0.0:8090",
  "state_dir": "state",
  "runtime_dir": "run",
  "muxes": []
}
```

| Field | Meaning |
| --- | --- |
| `version` | Required schema version, currently `1` |
| `listen` | Literal IP and TCP port; defaults to `0.0.0.0:8090` for LAN playback; IPv6 uses `[::1]:8090` syntax |
| `base_url` | Optional absolute player-facing URL, including any proxy path prefix |
| `state_dir` | Durable process lock, private tuner claims, and status snapshots |
| `runtime_dir` | Transient generated HLS; allocate enough disk or tmpfs capacity |
| `muxes[].name` | Unique lowercase name, up to 64 characters |
| `muxes[].device` | Specific eight-digit uppercase HDHomeRun hardware ID |
| `muxes[].tuner` | Zero-based tuner index; must exist on the device |
| `muxes[].rf` | US ATSC 1.0 physical channel, 2–36 |
| `muxes[].services[]` | Program number, name, optional virtual channel |

Relative directory paths resolve beside the config file, independently of your
current working directory. With `config/headend.json` and the default relative
paths, state is stored in `config/state/` and media in `config/run/`.
Unknown fields, invalid names, duplicate mux names,
duplicate services, and multiple muxes on one physical tuner are rejected. A
custom port must be between 1 and 65535. Device existence and tuner availability
are verified at runtime, not by the offline validator.

The transient layout is `runtime_dir/hls/MUX/PROGRAM/`. Startup removes only the
configured mux's old transient directory. Never place your own files in it.
Segments have six-entry sliding playlists with a four-second target duration;
actual durations depend on the incoming stream. Reserve memory/disk based on
your measured lineup bitrate. State and media roots must be private to the
installation and not writable by other users.

## Network access

The listener serves only the lineup, health check, and configured HLS files.
New configurations listen on all IPv4 interfaces at `0.0.0.0:8090`. To restrict
playback to the host, use `init --listen 127.0.0.1:8090` or change an existing
config's `listen` while stopped. Startup and `status` show the detected LAN IPs
and full `/lineup.m3u` URLs to enter in an IPTV player. With multiple active
networks, choose the address reachable from the player. The app serves HTTP
streams to connected players; it does not send multicast video.

There is no remote configuration API. Restrict the listener with your firewall
or place it behind your own authenticated reverse proxy. Do not forward its port
from your router to the public Internet.

To serve below a proxy path such as `/tv/`, forward that prefix to the origin's
root. Set `base_url` to the complete public prefix, for example
`https://media.example/tv`. Without an override, the served lineup contains
absolute channel URLs using the accepted connection's local IP and port. HTTP
headers from untrusted clients are never used to invent absolute playlist URLs.
An exported `playlist` uses the override or the first detected playback address.

## Systemd administration

For a complete installation walkthrough, see [Ubuntu service setup](ubuntu-service.md).
For foreground use on a Mac, see [macOS setup](macos.md).

The installer normalizes `state_dir` and `runtime_dir` for the service account.
It leaves listener, base URL, and mux selections intact. Do not copy foreground
tuner claim files into an installed service; stop the foreground instance cleanly
before switching. Retain the service's own durable state on later upgrades.

To add from a scan stored inside a private home directory:

```sh
sudo systemctl stop ota-headend
sudo install -o ota-headend -g ota-headend -m 0600 ./config/scan.json /var/lib/ota-headend/scan.json
sudo -u ota-headend ota-headend add --config /etc/ota-headend/config.json --scan /var/lib/ota-headend/scan.json --rf 11 --name second --tuner 1
sudo systemctl start ota-headend
```

Replace RF 11 with a scanned channel and choose an unused tuner. The config is
owned by the service account so the offline CLI can save it. systemd makes the
configuration directory read-only inside the running service.

An installed scan can also run while stopped:

```sh
sudo -u ota-headend ota-headend scan --config /etc/ota-headend/config.json --device 1234ABCD --tuner 0 --output /var/lib/ota-headend/scan-new.json
```

## Machine-readable output

`discover --json` returns device observations. `scan` writes a versioned JSON
document containing the device, tuner, timestamp, RF frequencies, signal values,
and services. Scan progress goes to stderr. `status --json` writes the status
document to stdout even if it then exits nonzero for stopped, stale, or unavailable
services. Capture logs go to stderr; `playlist` writes only M3U to stdout.
Running status snapshots include `playback`, an array of `ip` and `playlist_url`
pairs also shown by the human-readable `status` command. Addresses are captured
at startup; restart after changing the server's network addresses.
