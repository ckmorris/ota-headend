# Your local headend configuration

Use `config/headend.json` as the shared configuration for all devices, tuners,
and services. The repository includes a valid empty configuration with LAN
playback enabled at `0.0.0.0:8090`. No tuner is assigned until you scan and add
a multiplex. Run commands from the project root.
Read [the license](../LICENSE.md) and [end-user terms](../EULA.md) first;
`run --accept-terms` confirms your legitimate-use and anti-piracy agreement.

```sh
./bin/ota-headend doctor --config config/headend.json
./bin/ota-headend discover
./bin/ota-headend scan --config config/headend.json --device YOUR_DEVICE_ID --tuner 0 --output config/scan.json
./bin/ota-headend add --config config/headend.json --scan config/scan.json --rf YOUR_RF_CHANNEL --name local
./bin/ota-headend run --accept-terms --config config/headend.json
```

Replace the placeholders with a discovered hardware ID and a physical RF
channel from your scan. `add` includes all clear services on that multiplex
unless you select specific `--programs`. Repeat `add` with unique mux names and
unused device/tuner assignments to expand the same lineup.

Keep scan results here too, using a new filename for each scan. Relative
`state_dir` and `runtime_dir` paths resolve beside the JSON, so the supplied
config stores durable claims/status in `config/state/` and HLS in `config/run/`.
Scan files, state, generated media, and additional local configs are ignored by
Git. The starter `headend.json` is tracked; review changes before committing it.

If you have a previously configured file such as `wlfg.json`, stop the running
headend cleanly and wait for it to exit. Back up both configurations, then use
your existing configuration as `config/headend.json` instead of the empty starter.
Relative state/runtime paths now resolve inside `config/`. If the old process
crashed, recover its claims using the old config path before moving the file.

If `config/headend.json` is absent, create it without overwriting another config:

```sh
mkdir -p config
./bin/ota-headend init --config config/headend.json
```

When installing on Ubuntu, pass `--config ./config/headend.json` to the installer.
The installed service then uses `/etc/ota-headend/config.json`, with state in
`/var/lib/ota-headend` and HLS in `/run/ota-headend`. See the
[Ubuntu service guide](../docs/ubuntu-service.md) or [macOS guide](../docs/macos.md).
