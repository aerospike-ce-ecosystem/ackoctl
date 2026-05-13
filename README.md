# ackoctl

`ackoctl` is a command-line interface for [aerospike-cluster-manager](https://github.com/aerospike-ce-ecosystem/aerospike-cluster-manager), the management UI for Aerospike Community Edition clusters running on Kubernetes via [ACKO](https://github.com/aerospike-ce-ecosystem/aerospike-ce-kubernetes-operator).

It talks to cluster-manager's REST API (`/api/v1/*`) so that you can manage Aerospike connections, browse records, run queries, and trigger ACKO reconciliations from your terminal or CI pipeline — without leaving the shell.

## Status

**v0.1.0** — feature-complete for the control plane (connections, cluster info, k8s), data plane (records, sets), query and secondary-index management.

See [docs/usage.md](docs/usage.md) for a per-command cheat sheet and [docs/install.md](docs/install.md) for build and install options.

## Install

### Homebrew (macOS, Linux)

```bash
brew install aerospike-ce-ecosystem/tap/ackoctl
```

### Debian / Ubuntu (apt)

```bash
sudo install -d /etc/apt/keyrings
curl -fsSL https://aerospike-ce-ecosystem.github.io/ackoctl/key.gpg \
  | sudo gpg --dearmor -o /etc/apt/keyrings/ackoctl.gpg
echo "deb [signed-by=/etc/apt/keyrings/ackoctl.gpg] https://aerospike-ce-ecosystem.github.io/ackoctl/apt stable main" \
  | sudo tee /etc/apt/sources.list.d/ackoctl.list
sudo apt update && sudo apt install ackoctl
```

### RHEL / Fedora / Rocky (dnf/yum)

```bash
sudo curl -fsSL https://aerospike-ce-ecosystem.github.io/ackoctl/yum/ackoctl.repo \
  -o /etc/yum.repos.d/ackoctl.repo
sudo dnf install ackoctl
```

### Shell one-liner (no package manager)

```bash
curl -fsSL https://raw.githubusercontent.com/aerospike-ce-ecosystem/ackoctl/main/install.sh | sh
```

Detects OS/arch automatically (darwin/linux × amd64/arm64), verifies the sha256 checksum, and installs to `/usr/local/bin/ackoctl`. See [docs/install.md](docs/install.md) for pinning a version, custom `BIN_DIR`, manual install, and source build.

## Quick start

```bash
# Register a context pointing at a local cluster-manager (via kubectl port-forward)
ackoctl config set-context kind-local \
  --server=http://localhost:8000/api \
  --workspace-id=default
ackoctl config use-context kind-local
ackoctl config view
```

## Configuration

`ackoctl` reads `~/.ackoctl/config.yaml` (kubeconfig-style multi-context).

Override priority: CLI flag > environment variable > config file.

| Env var | Equivalent flag |
|---------|----------------|
| `ACKOCTL_SERVER` | `--server` |
| `ACKOCTL_TOKEN`  | `--token`  |
| `ACKOCTL_WORKSPACE` | `--workspace` |
| `ACKOCTL_CONTEXT` | `--context` |

OIDC tokens must be obtained out-of-band (e.g. via Keycloak CLI or browser device flow) and passed via `--token` or `ACKOCTL_TOKEN`.

## Commands

```
ackoctl
├── version
├── config       view | set-context | use-context | current-context | delete-context
├── connection   list | get | create | update | delete | health
├── cluster      info | configure-namespace
├── k8s cluster  list | get | reconcile
├── record       list | get | put | delete | query
├── set          list
├── query        exec
└── index        list | create | delete
```

See [docs/usage.md](docs/usage.md) for examples.

## Roadmap

- v0.2: admin (users/roles), UDF management, scriptable `--watch` flag
- v0.3: workspace CRUD, multi-cluster pivot helpers
- v1.0: stability promise after wider field testing

## Releasing

`ackoctl` follows SemVer. To cut a release:

```bash
# from a clean main branch
git tag v0.1.0
git push origin v0.1.0
```

Tagging triggers two workflows:

1. `release.yml` → goreleaser builds binaries + `.tar.gz` + `.deb` + `.rpm` + `.apk`, uploads them to the GitHub Release, and (with `GH_AW_GITHUB_TOKEN` set) bumps the formula in `aerospike-ce-ecosystem/homebrew-tap`.
2. `publish-packages.yml` (fires on `release.published`) → downloads the `.deb`/`.rpm` assets, rebuilds the APT + YUM repository metadata on the `gh-pages` branch, GPG-signs everything, and pushes. `apt install ackoctl` / `dnf install ackoctl` start serving the new version within a minute.

Required repository secrets:

| Secret | Purpose |
|--------|---------|
| `GH_AW_GITHUB_TOKEN` | PAT with `Contents: write` on `aerospike-ce-ecosystem/homebrew-tap` |
| `GPG_PRIVATE_KEY` | ASCII-armored private key (`gpg --armor --export-secret-keys`) for signing APT/YUM metadata and `.rpm` packages |
| `GPG_PASSPHRASE` | Passphrase for the above key (set empty if the key has none) |

One-time setup for the operator is documented in [docs/release-setup.md](docs/release-setup.md).

See [CHANGELOG.md](CHANGELOG.md) for release notes.

## License

Apache-2.0
