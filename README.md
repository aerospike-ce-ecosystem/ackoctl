# ackoctl

`ackoctl` is a command-line interface for [aerospike-cluster-manager](https://github.com/aerospike-ce-ecosystem/aerospike-cluster-manager), the management UI for Aerospike Community Edition clusters running on Kubernetes via [ACKO](https://github.com/aerospike-ce-ecosystem/aerospike-ce-kubernetes-operator).

It talks to cluster-manager's REST API (`/api/v1/*`) so that you can manage Aerospike connections, browse records, run queries, and trigger ACKO reconciliations from your terminal or CI pipeline — without leaving the shell.

## Status

**v0.2.0** — feature-complete for control-plane operations (connections, cluster info, K8s), data-plane reads/writes (records, sets, queries, secondary indexes), admin/UDF commands, operator notes, and self-upgrade.

See [docs/usage.md](docs/usage.md) for a per-command cheat sheet, [docs/install.md](docs/install.md) for build and install options, and [docs/e2e-kind.md](docs/e2e-kind.md) for an in-cluster (kind + ACKO + cluster-manager) end-to-end test scenario.

## Install

### Linux & macOS (one-liner)

```bash
curl -fsSL https://raw.githubusercontent.com/aerospike-ce-ecosystem/ackoctl/main/install.sh | sh
```

Detects OS/arch automatically (darwin/linux × amd64/arm64), verifies the sha256 checksum, and installs to `/usr/local/bin/ackoctl`. See [docs/install.md](docs/install.md) for pinning a version, custom `BIN_DIR`, manual install, and source build.

### Homebrew (macOS)

```bash
brew install aerospike-ce-ecosystem/tap/ackoctl
```

### Upgrade

Once `ackoctl` is on `$PATH` it can upgrade itself:

```bash
ackoctl upgrade           # pull the latest release
ackoctl upgrade --check   # report current vs latest, do not install
```

Every command also runs a once-a-day check against the GitHub Releases page and prints a one-line warning when a newer tag is available. Disable with `--no-version-check` or `ACKOCTL_NO_VERSION_CHECK=1`.

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
├── admin       user | role
├── cluster     info | configure-namespace
├── config       view | set-context | use-context | current-context | delete-context
├── connection   list | get | create | update | delete | health
├── guide        list | get
├── info         <connection-id> --command=<asinfo-command>
├── index        list | create | delete
├── k8s cluster  list | get | pods | logs | events | reconcile | scale
├── note         set | record
├── query        exec
├── record       list | get | put | delete | query
├── set          list
├── udf          list | upload | remove
├── upgrade
└── version
```

See [docs/usage.md](docs/usage.md) for examples.

## Roadmap

- v0.3: workspace CRUD, multi-cluster pivot helpers, scriptable `--watch` flag
- v1.0: stability promise after wider field testing

## Releasing

`ackoctl` follows SemVer. To cut a release:

```bash
# from a clean main branch
git tag v0.2.0
git push origin v0.2.0
```

Tagging triggers `release.yml` → goreleaser builds per-OS/arch `.tar.gz` archives and `checksums.txt`, uploads them to the GitHub Release alongside `install.sh`, and (with `GH_AW_GITHUB_TOKEN` set) bumps the formula in `aerospike-ce-ecosystem/homebrew-tap`.

Required repository secrets:

| Secret | Purpose |
|--------|---------|
| `GH_AW_GITHUB_TOKEN` | PAT with `Contents: write` on `aerospike-ce-ecosystem/homebrew-tap` |

One-time setup for the operator is documented in [docs/release-setup.md](docs/release-setup.md).

See [CHANGELOG.md](CHANGELOG.md) for release notes.

## License

Apache-2.0
