# Troubleshooting

Start with `ota-headend doctor --config config/headend.json`, `ota-headend discover`,
and `ota-headend status --config config/headend.json`. Use
`--config /etc/ota-headend/config.json` for an installed
service. For logs, use `sudo journalctl -u ota-headend -f`.

## No devices found

Check that the HDHomeRun has power and an Ethernet connection, and that the host
is on the same broadcast LAN. Guest Wi-Fi, VLAN isolation, Docker bridge
networks, VPN routes, and host firewalls can block discovery. Discovery uses
HDHomeRun's native tool, not an Internet directory. Test it directly:

```sh
hdhomerun_config discover
```

If the tool is missing, install the HDHomeRun command-line package. Discovery
does not need `tsp`; streaming needs both tools. Discovery may display zero
tuners when the device is visible but status probes fail. Do not treat that as
permission to control an unknown tuner.

## Tuner busy, tuned, or unreadable

Close the DVR/player or stop the other capture application that owns it. Choose
an idle tuner. OTA Headend checks `/lockkey`, `/target`, and complete tuner
status before claiming hardware; a device with another owner's lock or active
target is refused. It also refuses a tuner parked on a channel, even if that
channel matches the desired RF.

Do not issue force unlock commands as a normal setup step. They can interrupt
someone else's recording. There is intentionally no `--force` option.

If this installation crashed, preserve the state directory and run `recover`
while the service is stopped. Recovery uses the saved private key to authenticate
its own claim. Corrupt records, rejected keys, or unreadable devices fail without
clearing another owner's tuning. Restore a known-good state backup or resolve
the actual owning application before trying again.

## Scan has no stations

Confirm your antenna works with the HDHomeRun's own application. This application
only scans US ATSC 1.0 RF channels 2–36. It does not tune ATSC 3.0 or decrypt DRM.
Try a specific physical RF channel you know is active; a television's virtual
number is not necessarily its RF channel. Low-VHF channels 5 and 6 use 79 MHz
and 85 MHz center frequencies. Check signal quality as well as signal strength.

`config/scan.json` is a snapshot. Rescan after broadcasters move frequencies or program
numbers. Use a new output filename, and remove/re-add affected muxes while
stopped. A scan interrupted by Ctrl-C cleans up its tuner and removes its
incomplete result file; it does not silently save a partial lineup.

## HTTP 503 or services not ready

Wait 10–20 seconds after starting. A playlist needs two nonempty segments and
must be less than 20 seconds old. `status` also reports extractor errors and
programs absent from the current transport. Signal loss can leave a capture
process alive; stale HLS still returns 503.

If one program disappears, other services can remain available. Fix reception
or rescan the changed service, then restart the headend explicitly. Restarting
retries all configured muxes and briefly interrupts them; this release has no
automatic recovery loop or per-mux restart command.

If every service is unavailable, check `tsp -O hls --help`, dependency PATH,
hardware state, and disk space. HLS files are under the resolved
`runtime_dir/hls`; don't write your own content there. Each service owns a
sliding segment window, but high bitrates and many subchannels still require
adequate memory/disk capacity. This release has no automatic disk-quota manager.

## VLC works, browser does not

The application copies broadcast codecs. MPEG-2 video and AC-3 audio support
varies by player, device, and operating system. Use a compatible native player.
The application does not convert a station into H.264/AAC for browser playback.
The lineup is an IPTV M3U list; individual channel URLs are HLS playlists.

## Another computer cannot connect

New configurations default to `0.0.0.0:8090` for LAN playback. Run `status` and
copy the printed IPTV playlist URL into the player. For an older loopback-only
configuration, stop the headend, change `listen` to a LAN IP or `0.0.0.0:8090`,
then restart. Use the host's actual LAN IP in the player, not `0.0.0.0` or
`127.0.0.1`. If several addresses are printed, choose the player's LAN.
Permit TCP 8090 only from the required
trusted LAN/client addresses in your firewall. HDHomeRun capture also needs its
native LAN control/discovery and UDP stream traffic to reach the host.

The served M3U already includes absolute channel URLs for the address the player
connected to. For a reverse proxy, set config `base_url` to its reachable base
URL. If the computer's DHCP address changes, restart to refresh the addresses
shown in `status` and update the player's playlist URL, or use your own stable
DNS name. Update any explicit `base_url` too. Device assignments themselves use
hardware IDs.

## Permission denied or different results with systemd

Use the same config and durable state as the service. Foreground defaults and
the installed service are separate installations. Do not start both against the
same tuners. The service account needs read access to scan input files and write
access to its config directory for offline CLI edits. See
[systemd administration](configuration.md#systemd-administration).

The unit deliberately makes `/etc` read-only while the service runs. Make
changes after stopping it. External tools must be reachable in
`/usr/local/bin:/usr/bin:/bin`. Install matching native dependency packages for
your Linux CPU architecture; the static OTA Headend binary does not supply them.

## Reporting a problem

Send the version, OS/CPU, tuner model, tool versions, relevant status, and a short
log excerpt through the official GitHub repository. Remove private addresses or
identifying station information if desired. Never share `config/state/claims`, a saved
lock key, or an entire service state directory. Use the private security contact
for vulnerabilities rather than a public issue.
