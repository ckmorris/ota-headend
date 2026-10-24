# Install Go and choose a build target

Building OTA Headend requires Go 1.23 or later and `make`. A prebuilt Linux
release archive already contains the application binary, so running that
release does not require Go.

## macOS

With [Homebrew](https://brew.sh/) installed, install Go and verify it is on PATH:

```sh
brew install go
go version
```

The [official Homebrew Go formula](https://formulae.brew.sh/formula/go) installs
the toolchain for your Mac. If Go is already installed through Homebrew, update
it with `brew update` followed by `brew upgrade go`.

Alternatively, download the macOS `.pkg` installer from
[Go's downloads page](https://go.dev/dl/), choose Apple Silicon (`arm64`) or Intel
(`amd64`) to match your Mac, and open the package. Follow the installer, open a
new terminal, and run `go version`. See the [official installation guide](https://go.dev/doc/install).

macOS builds also require `make`, provided by Apple's Command Line Tools. If it
is missing, run `xcode-select --install`, complete the installer, and reopen
your terminal.

## Ubuntu

Use the current stable Linux archive from [Go's official downloads](https://go.dev/dl/).
The example below uses Go 1.27.3; update `go_release` to a newer stable version
when available. Choose `amd64` for an Intel/AMD 64-bit Ubuntu host, or `arm64`
for a 64-bit ARM host. `uname -m` reports `x86_64` or `aarch64`, respectively.

```sh
sudo apt update
sudo apt install ca-certificates curl make

go_release=1.27.3
go_arch=amd64
go_archive="go${go_release}.linux-${go_arch}.tar.gz"
curl -fLO "https://go.dev/dl/${go_archive}"
```

For ARM64 Ubuntu, set `go_arch=arm64` before setting `go_archive` and downloading.
Replace an existing manual installation in `/usr/local/go` before extracting;
the [Go installation guide](https://go.dev/doc/install) requires a clean tree:

```sh
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf "$go_archive"
export PATH="/usr/local/go/bin:$PATH"
go version
```

Persist the PATH entry for future login shells:

```sh
printf '%s\n' 'export PATH="/usr/local/go/bin:$PATH"' >> ~/.profile
```

The toolchain reported by `go version` must be at least 1.23. Ubuntu 24.04's
[default `golang-go` package](https://packages.ubuntu.com/noble/golang-go) is
older, so use the official archive above rather than relying on that package
for this project.

## Build for the machine you are using

From the project directory:

```sh
make build
./bin/ota-headend version
```

This writes a native binary to `bin/ota-headend`: a macOS executable on a Mac,
or a Linux executable on Ubuntu. Each build replaces that output file.

## Build on a Mac for Linux amd64 deployment

To replace `bin/ota-headend` with a Linux binary for an Intel/AMD 64-bit Ubuntu
server, run:

```sh
make build-linux-amd64
file bin/ota-headend
go version -m bin/ota-headend
```

The target runs the native build recipe with `CGO_ENABLED=0`, `GOOS=linux`, and
`GOARCH=amd64`. `file` should identify an ELF x86-64 executable; the Go build
metadata should show those target settings. Transfer it with the project and
configuration to your Ubuntu host, or supply its path through the Linux
installer's `--binary` option. Run it on the Linux server.

To switch `bin/ota-headend` back to a native Mac executable for local testing,
run `make build` again. Packaged releases built with `make release` contain
separate Linux amd64 and arm64 binaries; see [releasing.md](releasing.md).
