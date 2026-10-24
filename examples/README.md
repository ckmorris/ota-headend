# Example configurations

These files show the configuration format; users do not need to write JSON by
hand. Follow the [README setup flow](../README.md#first-channel-follow-along):
`init` creates the configuration, `discover` finds devices, `scan` discovers
received multiplexes and their services, and `add --scan ...` writes the chosen
multiplex into the configuration. Without `--programs`, `add` includes every
clear service reported by that scan. Repeat `add` for each desired multiplex,
choosing a different available device/tuner assignment for each.

- [config.json](config.json): a minimal illustrative multiplex with two services.
- [config.atlanta-7-tuners.json](config.atlanta-7-tuners.json): seven ATSC 1.0
  multiplexes in the Atlanta, Georgia market, using two four-tuner SiliconDust
  HDHomeRun devices. Four tuners are assigned on the first device and three on
  the second, with all 43 listed public television services on those transports.

The hardware IDs `1234ABCD` and `1234ABCE` are placeholders. Replace them with
your devices' IDs from `ota-headend discover`. Each device must support ATSC 1.0
on tuner indexes 0–3; tuner numbering starts at zero.

| Device placeholder | Tuner | Multiplex / source | Physical RF | Program → virtual channel |
| --- | --- | --- | --- | --- |
| `1234ABCD` | 0 | [WSB-TV](https://www.rabbitears.info/market.php?facility_id=23960&request=print_station) | 32 | 1→2.1, 2→2.2, 3→2.3, 4→2.4, 5→17.3, 6→2.5 |
| `1234ABCD` | 1 | [WAGA-TV](https://www.rabbitears.info/market.php?facility_id=70689&request=print_station) | 27 | 3→5.1, 4→5.2, 5→5.3, 6→5.4, 7→5.5, 8→5.6 |
| `1234ABCD` | 2 | [WXIA-TV](https://www.rabbitears.info/market.php?facility_id=51163&request=print_station) | 10 | 3→11.1, 4→17.2, 5→11.3, 6→11.4, 7→11.5, 9→11.7 |
| `1234ABCD` | 3 | [WPXA-TV](https://www.rabbitears.info/market.php?facility_id=51969&request=print_station) | 16 | 3→14.1, 4→14.2, 5→14.3, 6→14.4, 7→14.5, 8→14.6, 9→14.7, 10→14.8 |
| `1234ABCE` | 0 | [WGTV](https://www.rabbitears.info/market.php?facility_id=23948&request=print_station) | 7 | 1→8.1, 2→8.2, 3→8.3, 4→8.4 |
| `1234ABCE` | 1 | [WUVG-DT](https://www.rabbitears.info/market.php?facility_id=48813&request=print_station) | 18 | 1→34.1, 2→34.2, 3→34.3, 4→34.4, 5→34.5, 6→34.6, 7→34.7 |
| `1234ABCE` | 2 | [WATL](https://www.rabbitears.info/market.php?facility_id=22819&request=print_station) | 25 | 3→36.1, 4→36.2, 5→36.3, 6→11.11, 7→11.2, 8→36.4 |
| `1234ABCE` | 3 | Unassigned | — | — |

Each configured multiplex consumes one tuner, regardless of the number of
selected services. The unassigned eighth tuner is not claimed by this config
and can be used for scanning or by another application.

Station RF channels and program/virtual-channel mappings are based on the
[RabbitEars Atlanta listings](https://www.rabbitears.info/market.php?mktid=10&request=print_market),
consulted October 23, 2026, with per-station sources linked above. Program IDs
come from the physical-channel suffixes in their Streams tables, cross-checked
against available TSReader Program Association Tables, including
[WSB](https://www.rabbitears.info/screencaps/ga-atl/23960-0_0.htm),
[WAGA](https://www.rabbitears.info/screencaps/ga-atl/70689-0_0.htm), and
[WXIA](https://www.rabbitears.info/screencaps/ga-atl/51163-0_0.htm).
Use the MPEG program number, not the PSIP source ID or virtual-channel suffix.
For example, WXIA's virtual 11.1 is program 3, and its virtual 17.2 is program 4.

The example includes every listed public TV service on the seven selected RF
channels, including services carrying another station's virtual-channel prefix.
WGTV also lists program 8 (GPB Radio) without a virtual channel and flags it as
hidden or encrypted; it is omitted from this public television lineup.
WUVG's virtual channels 34.4–34.7 use MPEG-4 video, which the player must support.

This is a snapshot of published listings, not a live scan or a reception guarantee.
The linked transport captures are dated November 29, 2025; current station
listings supply the service labels. Setup scans obtain the actual received
programs for the user's location and hardware. These are ATSC 1.0 transports,
not the ATSC 3.0 simulcasts on other RF channels.

Validate the example without accessing tuners:

```sh
./bin/ota-headend validate --config examples/config.atlanta-7-tuners.json
./bin/ota-headend playlist --config examples/config.atlanta-7-tuners.json
```

For a normal installation, use the discovery/scan/add flow above to populate
`config/headend.json`. If you live in Atlanta, you can adapt this example by
using its JSON as `config/headend.json`, replacing both hardware IDs, and
confirming the RF/program mappings. Then start it with
`./bin/ota-headend run --config config/headend.json`. Running the headend claims
the seven assigned idle tuners; validation alone activates none.

The listener defaults to `0.0.0.0:8090` for LAN playback. Startup and `status`
display the actual LAN IP and M3U URL to load in an IPTV player.
Relative state and runtime directories resolve
beside the configuration file; see the [configuration guide](../docs/configuration.md).
The [config directory](../config/README.md) includes a valid empty starter for
normal scan-and-add setup.
