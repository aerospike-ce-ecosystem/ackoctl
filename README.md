# ackoctl

`ackoctl` is a command-line interface for [aerospike-cluster-manager](https://github.com/aerospike-ce-ecosystem/aerospike-cluster-manager), the management UI for Aerospike Community Edition clusters running on Kubernetes via [ACKO](https://github.com/aerospike-ce-ecosystem/aerospike-ce-kubernetes-operator).

It talks to cluster-manager's REST API (`/api/v1/*`) so that you can manage Aerospike connections, browse records, run queries, and trigger ACKO reconciliations from your terminal or CI pipeline — without leaving the shell.

## Status

**v0.1.0** — feature-complete for the control plane (connections, cluster info, k8s), data plane (records, sets), query and secondary-index management.

See [docs/usage.md](docs/usage.md) for a per-command cheat sheet and [docs/install.md](docs/install.md) for build and install options.

## Install

```bash
# macOS and Linux — same one-liner, no Homebrew required
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

The `release.yml` workflow runs goreleaser and publishes binaries to GitHub Releases (and, when `HOMEBREW_TAP_GITHUB_TOKEN` is configured, a Homebrew formula PR to `aerospike-ce-ecosystem/homebrew-tap`).

See [CHANGELOG.md](CHANGELOG.md) for release notes.

## License

Apache-2.0
