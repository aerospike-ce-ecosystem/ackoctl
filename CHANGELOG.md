# Changelog

All notable changes to ackoctl are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and ackoctl uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Security

- **`install.sh` no longer installs unverified binaries.** Previously a missing `checksums.txt`, a missing checksum entry, or the absence of `sha256sum`/`shasum` each downgraded to a warning and installed the binary anyway — and the missing-tool path even printed "checksum verified" without verifying anything. For a `curl … | sh` installer the threat model is exactly a tampered download, so all three paths are now fatal errors.
- **`config.Save` writes atomically with `0600` enforced.** The config file (which may hold bearer tokens) is now written through a temp file + atomic rename with `0600` permissions applied regardless of any pre-existing file mode, preventing credential exposure on an already-loose file and avoiding truncation on disk-full / interrupted writes.

### Changed

- **`query exec` predicate validation completed.** `--op between` now requires both `--value` and `--value2` client-side; `--value2` is rejected with any non-`between` operator; and a lone `--value2` is no longer silently dropped before predicate detection. These errors surface before the API call instead of as an opaque `422`.
- **`admin user create` / `admin user passwd` password flags.** `--password` and `--password-stdin` are marked mutually exclusive and one-of-required, so cobra prints usage when both or neither is supplied instead of a bare error string.

### Fixed

- **`ACKOCTL_NO_VERSION_CHECK` accepts any truthy value.** It previously recognised only `=1`; `=true` — the boolean grammar already used by `ACKOCTL_INSECURE_SKIP_TLS` — was silently ignored. It now parses via `strconv.ParseBool`.

## [0.2.0] — 2026-05-15

### Added

- **`ackoctl upgrade`** — in-place self-update. Resolves the latest tag from GitHub Releases, downloads the matching `tar.gz`, verifies the sha256 against `checksums.txt`, and atomically replaces the running binary. `--check` reports current vs. latest without installing; `--version vX.Y.Z` pins a specific release.
- **Startup version check** — every command (except `version`, `upgrade`, `help`, `completion`, and dev builds) consults a 24 h cache at `~/.ackoctl/.version-check.json` and prints a one-line stderr warning when a newer tag is available. Cache misses spawn a 1.5 s background refresh; opt out via `--no-version-check` or `ACKOCTL_NO_VERSION_CHECK=1`.

### Changed

- **`connection list` table now shows a `NOTE` column.** The connection profile's `note` was already returned by the API and visible in `-o json` / `-o yaml`, but the default table omitted it. Long bodies are truncated to 60 runes with an ellipsis (full text preserved in JSON/YAML).
- **Linux install path simplified.** The shell one-liner (`curl … install.sh | sh`) is now the only documented channel on Linux. The signed APT / YUM repositories on `gh-pages` are retired — `install.sh` already covers OS/arch detection, sha256 verification, and `~/.local/bin` fallback. Homebrew remains the macOS channel.
- **Goreleaser** — dropped the `nfpms` block; releases no longer ship `.deb` / `.rpm` / `.apk` artifacts. Only per-OS/arch `tar.gz` + `checksums.txt` + `install.sh` are uploaded.
- **Workflows** — `publish-packages.yml` (the `gh-pages` republish) and its dispatch step in `release.yml` removed. `GPG_PRIVATE_KEY` and `GPG_PASSPHRASE` repository secrets are no longer consumed by CI.

### Fixed

- **`docs/usage.md` admin role flag corrected.** Sample showed `admin role create ... --privileges=read.test,sindex-admin.test`; the actual cobra flag is `--privilege` (singular, repeatable) and `parsePrivileges` splits the scope on `:`. Corrected to `--privilege=read:test --privilege=sindex-admin:test`.
- **`internal/release/install.go` download timeout.** `downloadFile` previously used `http.DefaultClient` with no timeout. Now uses a dedicated `http.Client` bounded by a 5m `downloadTimeout`, matching the `internal/client` `defaultTimeout` pattern.
- **`gofmt` drift gate.** Added a `Check gofmt` step to the `lint` job in `.github/workflows/ci.yml` that fails when `gofmt -l .` is non-empty, and reformatted 17 previously-unformatted files.

### Migration

- `apt install ackoctl` / `dnf install ackoctl` users: reinstall via `curl -fsSL https://raw.githubusercontent.com/aerospike-ce-ecosystem/ackoctl/main/install.sh | sh`, then `sudo rm /etc/apt/sources.list.d/ackoctl.list /etc/apt/keyrings/ackoctl.gpg` (or the equivalent yum repo files) to silence stale `apt update` warnings.
- After this release, `ackoctl upgrade` is the recommended way to stay current. Homebrew users keep using `brew upgrade ackoctl`.

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
- Initial release was validated end-to-end against ACKO + cluster-manager deployed on a kind cluster: config / k8s / connection / cluster / record / set / index / query commands all exercised. Two schema mismatches surfaced and were fixed (`K8sClusterListResponse` envelope, pointer-field rendering).

[Unreleased]: https://github.com/aerospike-ce-ecosystem/ackoctl/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/aerospike-ce-ecosystem/ackoctl/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/aerospike-ce-ecosystem/ackoctl/releases/tag/v0.1.0
