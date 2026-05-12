# Installing ackoctl

## From source (recommended while ackoctl is pre-1.0)

```bash
git clone https://github.com/aerospike-ce-ecosystem/ackoctl.git
cd ackoctl
make build           # binary at ./bin/ackoctl
sudo mv ./bin/ackoctl /usr/local/bin/
ackoctl version
```

## go install

```bash
go install github.com/aerospike-ce-ecosystem/ackoctl/cmd/ackoctl@latest
```

This installs to `$(go env GOBIN)` (typically `~/go/bin`) — make sure that's on your `PATH`.

## Pre-built release binaries

Once a `v*` tag is pushed, GitHub Actions runs `goreleaser` and publishes `tar.gz` archives for:

- `darwin/amd64`, `darwin/arm64`
- `linux/amd64`, `linux/arm64`

Pick the archive that matches your machine on the [Releases page](https://github.com/aerospike-ce-ecosystem/ackoctl/releases) and untar it somewhere on your `PATH`.

```bash
curl -L https://github.com/aerospike-ce-ecosystem/ackoctl/releases/download/v0.1.0/ackoctl_0.1.0_darwin_arm64.tar.gz | tar -xz
sudo mv ackoctl /usr/local/bin/
```

## Homebrew (forthcoming)

The goreleaser config writes a Homebrew formula to `aerospike-ce-ecosystem/homebrew-tap` on every release.

```bash
brew tap aerospike-ce-ecosystem/tap
brew install ackoctl
```

If the tap doesn't exist yet, fall back to source or `go install` above — the tap repo and first release are bootstrapped together.

## Verifying

```bash
ackoctl version              # ackoctl version <SHA-or-tag>
ackoctl config view          # safe to run on a fresh install — prints empty config
```

## Updating

```bash
git -C "$(go env GOPATH)/src/github.com/aerospike-ce-ecosystem/ackoctl" pull
make build && sudo mv ./bin/ackoctl /usr/local/bin/
```

Or simply re-run `go install ...@latest`.
