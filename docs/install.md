# Installing ackoctl

`ackoctl` ships as a single static binary. Two install channels are supported: the shell one-liner (Linux, macOS) and Homebrew (macOS). Both serve the same artifacts from the GitHub Releases page.

## Shell one-liner (Linux, macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/aerospike-ce-ecosystem/ackoctl/main/install.sh | sh
```

The script:

- detects `darwin/linux` × `amd64/arm64` from `uname`,
- resolves the latest GitHub release (or honours `ACKOCTL_VERSION`),
- downloads the matching `tar.gz`, verifies the sha256 from `checksums.txt`,
- installs to `/usr/local/bin/ackoctl` (falls back to `$HOME/.local/bin` when not writable).

### Pin a version

```bash
ACKOCTL_VERSION=v0.1.0 \
  curl -fsSL https://raw.githubusercontent.com/aerospike-ce-ecosystem/ackoctl/main/install.sh | sh
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

If you'd rather skip the script, pick the archive that matches your machine from the [Releases page](https://github.com/aerospike-ce-ecosystem/ackoctl/releases) and untar it yourself:

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

The release also ships a `checksums.txt` — verify with `sha256sum -c` (Linux) or `shasum -a 256 -c` (macOS) before installing.

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

Installs to `$(go env GOBIN)` (typically `~/go/bin`).

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

`ackoctl` also prints a one-line warning to stderr when a newer release is available (checked once every 24h, results cached in `~/.ackoctl/.version-check.json`). Disable with `--no-version-check` or `ACKOCTL_NO_VERSION_CHECK=1`.

Homebrew users should use `brew upgrade ackoctl` instead so the formula stays in sync with the installed binary.

## Uninstalling

```bash
rm -f $(command -v ackoctl)
rm -rf ~/.ackoctl       # if you want to drop saved contexts as well
```
