# ackoctl

<p align="center">
  <img src="docs/images/logo.svg" alt="ackoctl" width="560" />
</p>

<p align="center">
  <a href="https://github.com/aerospike-ce-ecosystem/ackoctl"><img alt="GitHub repository" src="https://img.shields.io/badge/GitHub-ackoctl-0B1F33?logo=github&amp;logoColor=FFC72C"></a>
  <a href="https://github.com/aerospike-ce-ecosystem/ackoctl/actions/workflows/ci.yml"><img alt="CI status" src="https://img.shields.io/github/actions/workflow/status/aerospike-ce-ecosystem/ackoctl/ci.yml?branch=main&amp;logo=githubactions&amp;logoColor=FFC72C&amp;label=CI&amp;labelColor=0B1F33"></a>
  <a href="https://github.com/aerospike-ce-ecosystem/ackoctl/releases"><img alt="Latest release" src="https://img.shields.io/github/v/release/aerospike-ce-ecosystem/ackoctl?logo=github&amp;logoColor=FFC72C&amp;label=release&amp;labelColor=0B1F33&amp;color=647283"></a>
  <a href="LICENSE"><img alt="Apache 2.0 license" src="https://img.shields.io/badge/license-Apache%202.0-647283?logo=apache&amp;logoColor=FFC72C&amp;labelColor=0B1F33"></a>
</p>

`ackoctl` is a command-line client for [aerospike-cluster-manager](https://github.com/aerospike-ce-ecosystem/aerospike-cluster-manager). Cluster Manager manages Aerospike Community Edition clusters that run on Kubernetes through [ACKO](https://github.com/aerospike-ce-ecosystem/aerospike-ce-kubernetes-operator).

The CLI calls Cluster Manager's REST API (`/api/v1/*`). Use it from a terminal or CI pipeline to manage connections, browse records, run queries, and trigger ACKO reconciliations.

## Status

**v0.2.0** covers control-plane operations (connections, cluster information, and K8s), data-plane reads and writes (records, sets, queries, and secondary indexes), admin and UDF commands, operator notes, and self-upgrade.

See [docs/usage.md](docs/usage.md) for command examples and [docs/install.md](docs/install.md) for installation options. The [in-cluster test guide](docs/e2e-kind.md) walks through a kind + ACKO + Cluster Manager scenario.

## Install

### Linux & macOS (one-liner)

```bash
curl -fsSL https://raw.githubusercontent.com/aerospike-ce-ecosystem/ackoctl/main/install.sh | sh
```

The installer detects the OS and architecture (darwin/linux × amd64/arm64), verifies the SHA-256 checksum, and installs `ackoctl` in `/usr/local/bin`. See [docs/install.md](docs/install.md) to pin a version, choose `BIN_DIR`, install manually, or build from source.

### Homebrew (macOS)

```bash
brew install aerospike-ce-ecosystem/tap/ackoctl
```

### Upgrade

After `ackoctl` is on `$PATH`, it can upgrade itself:

```bash
ackoctl upgrade           # pull the latest release
ackoctl upgrade --check   # report current vs latest, do not install
```

Once a day, commands check GitHub Releases and print a one-line warning when a newer tag is available. Disable this check with `--no-version-check` or `ACKOCTL_NO_VERSION_CHECK=1`.

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
| `ACKOCTL_INSECURE_SKIP_TLS` | `--insecure-skip-tls` |

Obtain OIDC tokens outside `ackoctl`, for example with the Keycloak CLI or a browser device flow. Pass the token with `--token` or `ACKOCTL_TOKEN`.

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
├── record       list | get | put | delete | delete-bin | query
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

Pushing the tag starts `release.yml`. GoReleaser builds `.tar.gz` archives for each supported OS and architecture, generates `checksums.txt`, and uploads those files and `install.sh` to the GitHub Release. When `GH_AW_GITHUB_TOKEN` is set, the workflow also updates the formula in `aerospike-ce-ecosystem/homebrew-tap`.

Required repository secrets:

| Secret | Purpose |
|--------|---------|
| `GH_AW_GITHUB_TOKEN` | PAT with `Contents: write` on `aerospike-ce-ecosystem/homebrew-tap` |

One-time setup for the operator is documented in [docs/release-setup.md](docs/release-setup.md).

See [CHANGELOG.md](CHANGELOG.md) for release notes.

## License

Apache-2.0
