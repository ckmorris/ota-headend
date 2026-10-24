# Run OTA Headend on macOS

Run the foreground headend on a Mac connected to the same LAN as the HDHomeRun.
One configuration can include multiplexes from multiple tuners and devices;
one running headend serves all their services in one `/lineup.m3u`.

## Install and build

Install Go 1.23 or later and the runtime tools with [Homebrew](https://brew.sh/):

```sh
brew install go libhdhomerun tsduck
go version
```

For Go's macOS package installer, PATH setup, and Command Line Tools, see
[Install Go](install-go.md#macos).

The [libhdhomerun formula](https://formulae.brew.sh/formula/libhdhomerun) provides
`hdhomerun_config`. TSDuck's [macOS installation instructions](https://tsduck.io/docs/tsduck.html#_macos)
use Homebrew as well. Verify the tools are available:

```sh
command -v hdhomerun_config
tsp --version
tsp -O hls --help
```

After the repository is published, obtain it and build a native Mac binary:

```sh
git clone https://github.com/ckmorris/ota-headend.git
cd ota-headend
make build
```

If you already have a checkout, run `make build` there. Use a native build on
macOS; Linux release binaries do not run on a Mac.
For an Intel/AMD Ubuntu deployment, use `make build-linux-amd64` instead. This
replaces `bin/ota-headend` with a Linux executable; run `make build` again before
following the local Mac playback commands below.

## Discover, scan, and launch

The repository includes an empty `config/headend.json`. Use it for the entire
lineup, with scan results in the same directory:

```sh
./bin/ota-headend doctor --config config/headend.json
./bin/ota-headend discover
```

The listener defaults to `0.0.0.0:8090` for LAN playback. Scan with an idle
tuner, using the hardware ID shown by discovery:

```sh
./bin/ota-headend scan --config config/headend.json --device YOUR_DEVICE_ID --tuner 0 --output config/scan.json
./bin/ota-headend add --config config/headend.json --scan config/scan.json --rf YOUR_RF_CHANNEL --name local
./bin/ota-headend run --config config/headend.json
```

Replace the placeholders with your device ID and a physical RF channel found
by the scan. `add` inherits the scanned device and tuner and includes all clear
services on that RF channel. To add another multiplex, stop with Ctrl-C, repeat
`add` with another scanned RF, a unique name, and an unused `--tuner` assignment,
then run the same configuration again. A second device can be scanned separately
and added to the same file.

Use `--config config/headend.json` with `doctor`, `scan`, `add`, `run`, and
`status`. If the file is absent, create the directory with `mkdir -p config`
and run `./bin/ota-headend init --config config/headend.json` first. To adopt
an existing configuration, follow the [directory guide](../config/README.md).

Startup displays the LAN IP and complete **IPTV playlist** URL. Load that URL
into your IPTV player or VLC's **Media → Open Network Stream**. The headend
reports `Ready!` once all configured services have playable HLS output. Routine
capture dots are suppressed; capture diagnostics remain visible.

Keep the streaming terminal open. In another terminal in the project directory:

```sh
./bin/ota-headend status --config config/headend.json
```

If macOS requests local-network access or permission for incoming connections,
allow it for the headend and HDHomeRun tools. The player must be able to reach
the Mac's displayed LAN address on TCP 8090. Press Ctrl-C in the streaming
terminal to stop and release tuner claims.

The included installer and systemd unit are for Linux. For a headend that starts
at boot on Ubuntu, use the [Ubuntu service guide](ubuntu-service.md).
