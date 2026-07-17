# Installing ackoctl

`ackoctl` is a single static binary. Install it with the shell script on Linux or macOS, or with Homebrew on macOS. Both methods use artifacts from GitHub Releases.

## Shell one-liner (Linux, macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/aerospike-ce-ecosystem/ackoctl/main/install.sh | sh
```

The script does the following:

- detects `darwin/linux` × `amd64/arm64` from `uname`,
- resolves the latest GitHub release (or honours `ACKOCTL_VERSION`),
- downloads the matching `tar.gz`, verifies the sha256 from `checksums.txt`,
- installs to `/usr/local/bin/ackoctl` (falls back to `$HOME/.local/bin` when not writable).

### Pin a version

Set `ACKOCTL_VERSION` on the `sh` process, not on `curl`, so the pipe target receives the value:

```bash
curl -fsSL https://raw.githubusercontent.com/aerospike-ce-ecosystem/ackoctl/main/install.sh \
  | ACKOCTL_VERSION=v0.1.0 sh
```

Equivalent positional form when running the script directly:

```bash
curl -fsSL https://raw.githubusercontent.com/aerospike-ce-ecosystem/ackoctl/main/install.sh -o install.sh
sh install.sh v0.1.0
```

### Install to a custom directory

`BIN_DIR` must be set on the `sh` process — not on `curl`. Use either form:

```bash
curl -fsSL https://raw.githubusercontent.com/aerospike-ce-ecosystem/ackoctl/main/install.sh \
  | BIN_DIR="$HOME/.local/bin" sh
```

```bash
export BIN_DIR="$HOME/.local/bin"
curl -fsSL https://raw.githubusercontent.com/aerospike-ce-ecosystem/ackoctl/main/install.sh | sh
```

The script warns if `$BIN_DIR` is not on your `PATH`.

### Inspect before piping (recommended for paranoid environments)

```bash
curl -fsSL https://raw.githubusercontent.com/aerospike-ce-ecosystem/ackoctl/main/install.sh -o install.sh
less install.sh
sh install.sh
```

## Homebrew (macOS)

```bash
brew install aerospike-ce-ecosystem/tap/ackoctl
```

Homebrew handles updates: `brew upgrade ackoctl`. The formula lives in [aerospike-ce-ecosystem/homebrew-tap](https://github.com/aerospike-ce-ecosystem/homebrew-tap) and is bumped automatically on every release.

## Manual install

To install without the script, download the archive for your system from the [Releases page](https://github.com/aerospike-ce-ecosystem/ackoctl/releases) and extract it:

```bash
VERSION=0.1.0
OS=darwin   # darwin | linux
ARCH=arm64  # amd64  | arm64

curl -L -o ackoctl.tar.gz \
  "https://github.com/aerospike-ce-ecosystem/ackoctl/releases/download/v${VERSION}/ackoctl_${VERSION}_${OS}_${ARCH}.tar.gz"
tar -xzf ackoctl.tar.gz
sudo install -m 0755 ackoctl /usr/local/bin/ackoctl
ackoctl version
```

Each release includes `checksums.txt`. Verify it with `sha256sum -c` on Linux or `shasum -a 256 -c` on macOS before you install the binary.

## From source

```bash
git clone https://github.com/aerospike-ce-ecosystem/ackoctl.git
cd ackoctl
make build       # ./bin/ackoctl
sudo mv ./bin/ackoctl /usr/local/bin/
```

## go install

```bash
go install github.com/aerospike-ce-ecosystem/ackoctl/cmd/ackoctl@latest
```

This command installs the binary in `$(go env GOBIN)`, typically `~/go/bin`.

## Verifying

```bash
ackoctl version              # prints version, commit, build date, go runtime
ackoctl config view          # safe on a fresh install — prints an empty config
```

## Updating

```bash
ackoctl upgrade              # in-place self-update; verifies sha256 before swap
ackoctl upgrade --check      # report current vs latest, do not install
ackoctl upgrade --version v0.1.0   # pin to a specific release
```

When a newer release is available, `ackoctl` prints a one-line warning to stderr. It checks once every 24 hours and caches the result in `~/.ackoctl/.version-check.json`. Disable the check with `--no-version-check` or `ACKOCTL_NO_VERSION_CHECK=1`.

Homebrew users should use `brew upgrade ackoctl` instead so the formula stays in sync with the installed binary.

## Uninstalling

```bash
rm -f $(command -v ackoctl)
rm -rf ~/.ackoctl       # if you want to drop saved contexts as well
```
