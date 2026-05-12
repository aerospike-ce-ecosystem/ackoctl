# Changelog

All notable changes to ackoctl are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and ackoctl uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] — 2026-05-12

First public release. Feature-complete coverage of the cluster-manager REST surface for everyday control-plane and data-plane work.

### Added

- **Skeleton** — cobra root with persistent flags (`--context`, `--server`, `--token`, `--workspace`, `-o`, `-v`, `--insecure-skip-tls`); ldflags-injected `version`; kubeconfig-style multi-context config at `~/.ackoctl/config.yaml` with `flag > env > file` override precedence; `config view/set-context/use-context/current-context/delete-context`.
- **Connections** — `connection list/get/create/update/delete/health`. Multi-host seeds via repeatable `--host`; label key=value support; workspace propagation from context.
- **Cluster** — `cluster info` (raw map pass-through, polished table fallback); `cluster configure-namespace` for runtime-mutable knobs (Aerospike does not allow creating namespaces at runtime).
- **Kubernetes** — `k8s cluster list/get/reconcile` against ACKO-managed `AerospikeCluster` CRs. `reconcile` stamps `acko.io/force-reconcile` via cluster-manager's `/force-reconcile` endpoint.
- **Records** — `record list/get/put/delete/query`. `put` accepts `--bins` as a JSON object; `query` exposes the full `FilterGroup` / `QueryPredicate` DSL via `--filter` / `--predicate` raw JSON; pk-pattern matching with `--pk-match-mode prefix|regex`.
- **Sets** — `set list` derived from cluster info (no dedicated server endpoint); tolerates `objects` / `object_count` and `memUsed` / `memory_used` / `data-used-bytes` aliases.
- **Query** — `query exec` supporting predicate (`--bin/--op/--value[/--value2]`), primary-key lookup, or full scan with `--max-records`. `--value`/`--value2` are JSON-parsed with string fallback.
- **Indexes** — `index list/create/delete` for `numeric | string | geo2dsphere`. Destructive `delete` requires `--yes`.
- **Output formatting** — `-o table|json|yaml` everywhere. Polished raw-map table fallback (sorted keys, nested struct/map/slice handling, empty-slice rendering).
- **Release pipeline** — Makefile, golangci-lint, GitHub Actions (test + lint + cross-build matrix), goreleaser for darwin/linux × amd64/arm64 with sha256 checksums and an `install.sh` curl one-liner that works identically on macOS and Linux.
- **Docs** — `docs/usage.md` cheat sheet, `docs/install.md` install methods, README command tree.

### Notes

- `ackoctl` has no `login` flow. Obtain bearer tokens out-of-band (Keycloak CLI, browser device flow, etc.) and inject via `--token`, `ACKOCTL_TOKEN`, or `config set-context --token=...`.
- `k8s` commands require cluster-manager to be started with `K8S_MANAGEMENT_ENABLED=true`. Without it, every `k8s` request returns HTTP 404.
- Workspace ACL is explicit: every resource command honors `--workspace`, falling back to the current context's `workspace-id`. There is no silent "first workspace" default.

[Unreleased]: https://github.com/aerospike-ce-ecosystem/ackoctl/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/aerospike-ce-ecosystem/ackoctl/releases/tag/v0.1.0
