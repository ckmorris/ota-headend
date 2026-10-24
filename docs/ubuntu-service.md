# Run OTA Headend as an Ubuntu service

One configuration contains all selected multiplexes across all HDHomeRun devices.
One `ota-headend run` process starts every configured service and serves them all
in one M3U at `http://SERVER_LAN_IP:8090/lineup.m3u`.

These instructions use Ubuntu 24.04 or later with systemd. Run the commands on
the Ubuntu host that will capture and serve the streams, on the tuners' LAN.

## 1. Install dependencies and prepare the Linux binary

```sh
sudo apt update
sudo apt install hdhomerun-config ca-certificates python3 make
```

[hdhomerun-config](https://packages.ubuntu.com/noble/hdhomerun-config) is in
Ubuntu's Universe repository; enable Universe if APT cannot find it.
Download the TSDuck runtime `.deb` matching your Ubuntu release and CPU from
the [official downloads](https://tsduck.io/tsduck-binaries/). In a directory
containing just that runtime package, run:

```sh
sudo apt install ./tsduck_*.deb
```

If building on Ubuntu, first [install Go 1.23 or later](install-go.md#ubuntu).
That guide includes the download, extraction, PATH setup, and version check;
Ubuntu 24.04's default Go package is older than the project's minimum.
Copy the project directory and your `config/headend.json` to the Ubuntu host.
Inside that project directory, build:

```sh
go version
make build
```

Alternatively, use a Linux release archive matching the host's CPU; its binary
is already in `bin/ota-headend` and requires no Go installation. You can also
build on your Mac for Intel/AMD Ubuntu with `make build-linux-amd64`, then
transfer the resulting `bin/ota-headend` with the project to Ubuntu. This target
sets `GOOS=linux`, `GOARCH=amd64`, and `CGO_ENABLED=0`; a native macOS build from
`make build` cannot run on Ubuntu.

## 2. Prepare one combined configuration

The repository includes a valid empty `config/headend.json`. Use it as the one
configuration for all multiplexes. If you already configured stations in a file
such as `wlfg.json`, adopt it using the [directory guide](../config/README.md).
To add a multiplex, stop the foreground headend with Ctrl-C, scan with an idle
tuner, then append to that same configuration:

```sh
./bin/ota-headend discover
./bin/ota-headend scan --config config/headend.json --device YOUR_DEVICE_ID --tuner 0 --output config/scan-next.json
./bin/ota-headend add --config config/headend.json --scan config/scan-next.json --rf YOUR_RF_CHANNEL --name next --tuner 0
```

Replace the device and RF placeholders with a discovered device and a physical
channel reported by the scan. Choose an unused tuner and a unique mux name.
Tuner 0 above is an example; preserve any existing tuner assignments. Without
`--programs`, `add` imports all clear services on the selected RF channel.
Repeat `add` for further multiplexes, using a distinct device/tuner pair each
time. Scan files are setup inputs, not separate configurations to launch.

If the configuration file is absent, run `mkdir -p config` and then
`./bin/ota-headend init --config config/headend.json` before scanning. Keep local
scan results in `config/` too. Use a new scan filename each time.

Validate and test the combined configuration:

```sh
./bin/ota-headend doctor --config config/headend.json
./bin/ota-headend validate --config config/headend.json
./bin/ota-headend run --accept-terms --config config/headend.json
```

Startup prints the LAN IP and combined M3U URL. It reports `Ready!` when all
configured services have playable HLS output. Test playback, then press Ctrl-C
and wait for the process to exit before installing the service. Transfer only
configuration when moving from another host; do not copy tuner claim files.

## 3. Install and enable the service

From the project directory, after reading [LICENSE.md](../LICENSE.md) and
[EULA.md](../EULA.md), including the legitimate-use and anti-piracy agreement:

```sh
sudo ./scripts/install-linux.sh --config ./config/headend.json --accept-license
sudo systemctl enable --now ota-headend
```

The installer creates an unprivileged `ota-headend` account, installs the binary
and the included systemd unit, and copies your combined configuration to
`/etc/ota-headend/config.json` on the first installation. It normalizes durable
state to `/var/lib/ota-headend` and transient HLS to `/run/ota-headend`.
The service runs:

```text
/usr/local/bin/ota-headend run --config /etc/ota-headend/config.json --accept-terms
```

`enable --now` starts it immediately and enables startup at boot; see
[systemctl's documentation](https://www.freedesktop.org/software/systemd/man/latest/systemctl.html).
You can close your terminal after enabling the service.
The installer's `--accept-license` confirms agreement to both the software
license and end-user terms. The installed unit includes `--accept-terms` for
unattended startup after that acceptance. When upgrading from the initial
release, run the installer to update the unit before starting the new binary.

On subsequent installs, the installer preserves the existing
`/etc/ota-headend/config.json`. The supplied `--config` does not replace that
installed configuration. Once installed, make all lineup changes against the
file in `/etc`, using the commands below.

## 4. Check status and connect your IPTV player

```sh
sudo systemctl status ota-headend --no-pager
sudo journalctl -u ota-headend -n 50 --no-pager
sudo -u ota-headend ota-headend status --config /etc/ota-headend/config.json
```

Copy the full **IPTV playlist** URL into your player's M3U URL field. It contains
every configured service from every configured tuner; no manual playlist merging
is needed. Allow about 10–20 seconds for HLS to become ready. To follow logs:

```sh
sudo journalctl -u ota-headend -f
```

If UFW is already enabled, permit the actual LAN subnet to access TCP 8090.
For example, for a `192.168.1.0/24` LAN:

```sh
sudo ufw allow from 192.168.1.0/24 to any port 8090 proto tcp
```

Use your own subnet. New configs listen on `0.0.0.0:8090`; the player uses the
actual LAN IP printed in the logs or status.

## 5. Add multiplexes after installation

Stop the service, scan with a free tuner, and append to the installed config:

```sh
sudo systemctl stop ota-headend
sudo -u ota-headend ota-headend scan --config /etc/ota-headend/config.json --device YOUR_DEVICE_ID --tuner 0 --output /var/lib/ota-headend/scan-next.json
sudo -u ota-headend ota-headend add --config /etc/ota-headend/config.json --scan /var/lib/ota-headend/scan-next.json --rf YOUR_RF_CHANNEL --name next --tuner 0
sudo -u ota-headend ota-headend validate --config /etc/ota-headend/config.json
sudo systemctl start ota-headend
```

Use real device/RF values, an unused tuner, and a unique mux name. Give each scan
a new output filename. Use `remove --config /etc/ota-headend/config.json --name
MUX_NAME` as the service account while stopped to remove a multiplex.
All changes appear in the same M3U after starting the service again.

For a binary upgrade, stop the service, run the installer again, then start it.
The installer preserves the installed config and durable state. Do not run a
foreground instance alongside the service against the same tuners.
