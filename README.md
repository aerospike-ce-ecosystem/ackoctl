# ackoctl

`ackoctl` is a command-line interface for [aerospike-cluster-manager](https://github.com/aerospike-ce-ecosystem/aerospike-cluster-manager), the management UI for Aerospike Community Edition clusters running on Kubernetes via [ACKO](https://github.com/aerospike-ce-ecosystem/aerospike-ce-kubernetes-operator).

It talks to cluster-manager's REST API (`/api/v1/*`) so that you can manage Aerospike connections, browse records, run queries, and trigger ACKO reconciliations from your terminal or CI pipeline — without leaving the shell.

## Status

**P0 (skeleton)** — `version` and `config` subcommands. Resource commands land in P1+.

## Install

```bash
git clone https://github.com/aerospike-ce-ecosystem/ackoctl.git
cd ackoctl
make build
./bin/ackoctl version
```

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

## Roadmap

- P1: `connection`, `cluster`, `k8s` subcommands (control plane)
- P2: `record` CRUD + `set list` (data plane)
- P3: `query exec`, `index` CRUD
- P4: release artifacts (brew, goreleaser), asc-workspace submodule integration

## License

Apache-2.0
