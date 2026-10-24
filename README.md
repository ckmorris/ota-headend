# OTA Headend

Turn an antenna-connected HDHomeRun into an always-on HLS source for your home
network. Discover your tuners, scan your local stations, select the RF
multiplexes you want, and open the resulting channel lineup in a compatible
player. One physical tuner receives one RF multiplex and serves all selected
subchannels on it, regardless of how many people watch.

**By Chris Morris.** Licensed under
[PolyForm Noncommercial 1.0.0](LICENSE.md). Noncommercial use, modification,
forks, and redistribution are permitted under its terms. Commercial use outside
the license's permitted purposes requires a separate paid written license from
Chris Morris. This is source-available software, not an open-source license.
See the [end-user terms and licensing summary](EULA.md) for details.

**Legitimate use only.** OTA Headend is released for lawful use, and the project
does not condone piracy. Users must agree not to use it for copyright
infringement, unauthorized access, unlawful retransmission or redistribution,
or unlawful circumvention of access controls. Read [the anti-piracy agreement](EULA.md#legitimate-use-and-anti-piracy-agreement)
before using this release. Starting the headend requires `--accept-terms` to
confirm that agreement. Software licensing grants no rights in received media;
lawful uses and statutory exceptions remain available.

This standalone application uses the newer internal worker’s HDHomeRun/TSDuck
capture design. It runs locally without an account, Rails server,
enrollment token, cloud service, GPU, FFmpeg, or Nginx. Setup does not depend on
the developer's channel lineup or network addresses.

## What you need

- A Linux computer or Mac connected to the same LAN as your tuner, preferably
  over Ethernet. Ubuntu 24.04 is the reference service installation target;
  macOS supports foreground use with Homebrew dependencies.
- An HDHomeRun that supports **ATSC 1.0 / 8VSB**, connected to a working antenna.
  This release scans US physical RF channels 2–36. ATSC 3.0, DRM, cable/QAM,
  DVB, USB tuners, and non-US frequency plans are outside this release's scope.
- An idle tuner for scanning and one tuner per RF multiplex you keep streaming.
  A four-tuner device can continuously receive four different RF multiplexes.
- `hdhomerun_config` and TSDuck's `tsp`, installed on the streaming computer.
- A player that supports HLS with the station's original video/audio codecs.
  VLC is a useful first test. Many browsers cannot play MPEG-2 video or AC-3
  audio even when the HLS is valid.

Streams retain the broadcast codecs and bitrate. There is no transcoding,
adaptive bitrate, DVR, program guide ingestion, or fallback video slate. Expect
several seconds of buffering. The HLS origin has no login or TLS; bind it to
localhost or keep access within a trusted LAN.

## Install

For macOS, follow the [Homebrew setup guide](docs/macos.md). For an Ubuntu
service that starts at boot, follow the [Ubuntu service guide](docs/ubuntu-service.md).

Obtain the source or release archive from the official project repository on the
[ckmorris GitHub account](https://github.com/ckmorris). Downloading and running a
release does not require a GitHub account.

### Dependencies on Ubuntu

```sh
sudo apt update
sudo apt install hdhomerun-config ca-certificates
```

Download the TSDuck runtime `.deb` matching **your Ubuntu release and CPU** from
the [official TSDuck downloads](https://tsduck.io/tsduck-binaries/). From the
directory containing that package, install it with APT so dependencies are
resolved:

```sh
sudo apt install ./tsduck_*.deb
```

Use a directory containing only the runtime package you intend to install.
For another distribution or architecture, follow the
[official TSDuck build and installation guide](https://tsduck.io/docs/tsduck-build.html).
These external programs are installed separately and retain their own licenses.

Check that the tools are on your PATH:

```sh
command -v hdhomerun_config
tsp --version
tsp -O hls --help
```

### Build from source

Install Go 1.23 or later using the [macOS and Ubuntu Go installation guide](docs/install-go.md)
if you are building the application yourself. Inside the downloaded project directory:

```sh
make build
./bin/ota-headend --help
```

`make build` creates a binary for the machine running the build. To build on a
Mac for an Intel/AMD 64-bit Ubuntu server and replace `bin/ota-headend` with a
Linux amd64 executable:

```sh
make build-linux-amd64
file bin/ota-headend
```

Run that binary on the Linux server. To rebuild for local Mac use, run
`make build` again. See the [build target instructions](docs/install-go.md#build-on-a-mac-for-linux-amd64-deployment).

The application has no third-party Go module dependencies. A release archive
already contains `bin/ota-headend`; choose an archive matching your Linux CPU
(`amd64` for most PCs, `arm64` for 64-bit ARM). You do not need Go to run it.
Verify the archive against the publisher's `SHA256SUMS` before extracting it.

The commands below use `./bin/ota-headend` so you can follow along without
installing anything system-wide. Keep the same terminal in this project directory.
Discovery, scanning, and foreground playback do not need `sudo`.

## First channel: follow along

### 1. Check your configuration

The repository includes a valid empty [config/headend.json](config/headend.json).
Use this one file for every device and multiplex. Keep scan results in `config/`
as well; see [the directory guide](config/README.md).

```sh
./bin/ota-headend doctor --config config/headend.json
```

If the file is absent, create it first:

```sh
mkdir -p config
./bin/ota-headend init --config config/headend.json
```

`init` refuses to overwrite an existing file. The default HTTP listener is
`0.0.0.0:8090`, which serves playback on all IPv4 interfaces, including your LAN.
`doctor` checks dependencies and configuration
without tuning anything.

For playback restricted to this computer, set `listen` to `127.0.0.1:8090` in
`config/headend.json` while the headend is stopped. An empty `muxes` list is
valid and safe to validate; add at least one scanned multiplex before running.

### 2. Discover your HDHomeRun

```sh
./bin/ota-headend discover
```

You will see hardware IDs, current addresses, models, tuner counts, and observed
tuner state. Discovery issues only read commands. Copy your device's eight-digit
hardware ID and choose an idle tuner; indexes start at **0**. The examples below
use the illustrative ID `1234ABCD`. Replace it with yours.

Do not reuse a tuner currently used by a DVR or another application. A displayed
RF lock describes reception, not exclusive ownership; the scan and streaming
commands perform additional ownership checks before making changes.

### 3. Scan for stations

```sh
./bin/ota-headend scan --config config/headend.json --device 1234ABCD --tuner 0 --output config/scan.json
```

The scan temporarily claims that tuner, scans RF 2–36, reports each discovered
program, writes `config/scan.json`, and releases the tuner. Allow roughly 90 seconds
plus network overhead. Your configuration stays unchanged. The output file must
be new; use a different filename for a later scan.

For a quicker scan of known physical channels:

```sh
./bin/ota-headend scan --config config/headend.json --device 1234ABCD --tuner 0 --rf 7,11,19-22 --output config/scan-selected.json
```

A result might look like this:

```text
Scanning RF 7...
  RF 7 / program 3 / 5.1 Example TV
  RF 7 / program 4 / 5.2 Example Two
```

**RF 7** is the physical multiplex. **5.1** is the virtual channel shown on a
television. **Program 3** identifies a service within the multiplex. Use the
scan's RF number for `--rf`; do not substitute the virtual channel number.

### 4. Choose a multiplex

Using an RF channel actually found in your scan:

```sh
./bin/ota-headend add --config config/headend.json --scan config/scan.json --rf 7 --name local
./bin/ota-headend validate --config config/headend.json
```

This adds all discovered clear services on RF 7, using the device and tuner from
the scan. It saves configuration and starts no capture. `local` is a name you
choose: lowercase letters, digits, dashes, and underscores are accepted.

To select only particular program numbers instead, use this form when adding:

```sh
./bin/ota-headend add --config config/headend.json --scan config/scan.json --rf 7 --name local --programs 3,4
```

Run one add command for a given mux name, not both. Use
`remove --config config/headend.json --name local`
while stopped if you want to replace it.

### 5. Start streaming

```sh
./bin/ota-headend run --accept-terms --config config/headend.json
```

Leave that terminal open. In another terminal, from the same directory:

```sh
./bin/ota-headend status --config config/headend.json
curl -f http://127.0.0.1:8090/healthz
```

Allow about 10–20 seconds for initial HLS segments. Status shows each service
independently. `status` exits with an error while a service is unavailable, when
the headend is stopped, or if the snapshot is stale. `/healthz` returns 200 only
when every configured service is ready, otherwise 503.
Startup reports `Ready!` once all configured services have playable HLS output.
Routine capture dots are suppressed while the server runs; errors remain visible.

Startup and `status` display the server's LAN IP and an **IPTV playlist** URL
for each active LAN address. Load that complete URL into your IPTV program's
M3U playlist field, or VLC's **Media → Open Network Stream**. For example:

```text
http://192.0.2.10:8090/lineup.m3u
```

Use the actual URL printed by your server; `192.0.2.10` is illustrative.
`0.0.0.0` is a bind address, not the address to enter in a player. The URL serves
the current channel listings directly; no manual M3U download is needed.
When several addresses are shown, choose the one on your player's LAN.

You can also open an individual service on that server:

```text
http://192.0.2.10:8090/hls/local/3/index.m3u8
```

Here `3` is its MPEG program number. The served M3U contains absolute channel
URLs using the address through which the player connected. For a reverse proxy,
set `base_url` in config to its public base URL. To save a standalone M3U file:

```sh
./bin/ota-headend playlist --config config/headend.json --base-url http://192.0.2.10:8090 > channels.m3u
```

Replace `192.0.2.10` with the streaming computer's LAN address. For LAN playback,
your firewall must allow the player's connection to TCP 8090. Without
`--base-url`, the `playlist` command uses the configured `base_url`, or the first
detected playback address. Local playback also works at
`http://127.0.0.1:8090/lineup.m3u` with the default listener.

### 6. Stop cleanly

Press **Ctrl-C** in the streaming terminal. The headend stops its child processes,
clears only its owned tuner targets and tuning, and releases its lock keys.
Stopping playback in VLC leaves capture running for other viewers.

## Add more stations

Stop the foreground headend first. A running installation holds a process lock
and rejects scans and config edits through the CLI.

**One configuration, one process, one combined M3U.** Each `add` appends a
multiplex and its services to the same config. A single `run` launches all of
them, including assignments on different devices. The `/lineup.m3u` endpoint
contains every configured service; scan files are only inputs during setup.

Use your saved scan to assign another RF multiplex to another idle tuner:

```sh
./bin/ota-headend add --config config/headend.json --scan config/scan.json --rf 11 --name second --tuner 1
./bin/ota-headend run --accept-terms --config config/headend.json
```

Replace RF 11 with a channel found by your scan. A tuner cannot be assigned twice.
To use another HDHomeRun, discover and scan that device, then add from its scan
file. Hardware IDs identify devices across DHCP address changes.
Keep using `--config config/headend.json` for every `scan`, `add`, `run`, and
`status` command. To adopt an existing file such as `wlfg.json`, follow the
[configuration directory guide](config/README.md).

Configuration reference, layout, and command details:
[docs/configuration.md](docs/configuration.md).

## Run at boot on Linux

See the [step-by-step Ubuntu service guide](docs/ubuntu-service.md) for
dependencies, building on Ubuntu, installing `config/headend.json`, and adding
more multiplexes after installation.

Once foreground playback works, stop it and install the optional systemd service:

Read LICENSE.md and EULA.md first. The installer's `--accept-license` confirms
agreement to both, including the legitimate-use and anti-piracy terms.

```sh
sudo ./scripts/install-linux.sh --config ./config/headend.json --accept-license
sudo systemctl enable --now ota-headend
sudo journalctl -u ota-headend -f
```

The installer creates an unprivileged service account, installs the binary and
unit, and copies the initial config with durable state in
`/var/lib/ota-headend` and transient HLS in `/run/ota-headend`.
It does not install dependencies or start the service. On upgrade it preserves
the existing `/etc/ota-headend/config.json`.

Once installed, use the installed config for all administrative commands:

```sh
sudo -u ota-headend ota-headend status --config /etc/ota-headend/config.json
sudo systemctl stop ota-headend
sudo -u ota-headend ota-headend add --config /etc/ota-headend/config.json --scan "$PWD/config/scan.json" --rf 11 --name second --tuner 1
sudo systemctl start ota-headend
```

The service account must be able to read the scan file and traverse its parent
directories. If your home directory is private, copy the scan into the service's
state directory and give it to `ota-headend` first. See the
[configuration guide](docs/configuration.md#systemd-administration) for that flow.

## Recovery and troubleshooting

The application saves private ownership records **before** claiming hardware.
After a crash, the next `run` or `scan` authenticates those saved claims before
releasing them. You can explicitly recover while stopped:

```sh
./bin/ota-headend recover --config config/headend.json
```

Never delete `config/state/` to fix a tuner lock. Unknown locks are refused; the program
does not force unlocks or take over other applications' tuners. Preserve the
installed state directory across upgrades.

Failed services return HTTP 503 instead of serving a stale playlist. Other
working services continue if one extractor exits. A failed multiplex or missing
service requires an explicit headend restart to retry. There is no automatic
retune/retry or per-multiplex restart command in this release.

For no devices, busy tuners, no stations, codec issues, firewall setup, or crash
recovery, see [docs/troubleshooting.md](docs/troubleshooting.md).

## Project and release information

- [macOS setup with Homebrew](docs/macos.md)
- [Install Go and choose a build target](docs/install-go.md)
- [Ubuntu system service setup](docs/ubuntu-service.md)
- [Architecture and extraction notes](docs/architecture.md)
- [Validation and release checklist](docs/releasing.md)
- [Contributing](CONTRIBUTING.md) and [contributor agreement](CLA.md)
- [Security reporting](SECURITY.md)
- [License](LICENSE.md), [end-user terms and licensing summary](EULA.md), and
  [third-party notices](THIRD_PARTY_NOTICES.md)

For commercial use outside the permissions in LICENSE.md, contact Chris Morris
through the official project repository on
[GitHub](https://github.com/ckmorris). Obtain a separate written commercial
license and pay the agreed fee before that use. Opening an inquiry is not
permission. Chris Morris controls contributions and releases in the official
project; permitted forks do not imply endorsement.
