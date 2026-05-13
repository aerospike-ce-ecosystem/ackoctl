# Release infrastructure setup

The `release.yml` workflow is fully automated. The initial scaffolding (Homebrew tap, cross-repo PAT) is already in place — this document records the current state and the few operator-facing knobs that remain.

Every `git push --tags` will:

1. Build per-OS/arch `.tar.gz` archives + `checksums.txt` via goreleaser
2. Upload them (plus `install.sh`) to the GitHub Release page
3. Bump the Homebrew formula in `aerospike-ce-ecosystem/homebrew-tap`

…with no manual steps. Users install via the upstream one-liner (`curl … | sh`) or Homebrew — no package-manager repos (apt/yum) are hosted.

---

## Current state

| Resource | Status |
|----------|--------|
| `aerospike-ce-ecosystem/homebrew-tap` repository | created (public, `main` branch, `Formula/ackoctl.rb` bumped on each release) |
| `GH_AW_GITHUB_TOKEN` (org/repo secret, used by goreleaser to push the formula across repos) | present |

---

## Reference: re-creating the scaffolding from scratch

If you ever need to rebuild from zero, the steps below are the same ones the initial setup followed.

## 1. Create the Homebrew tap repository

| Field | Value |
|-------|-------|
| Repo  | `aerospike-ce-ecosystem/homebrew-tap` |
| Visibility | Public |
| Initial content | a single `README.md` is enough |

The repository name **must** start with `homebrew-` (Homebrew convention). The user-facing tap name (`aerospike-ce-ecosystem/tap`) drops that prefix automatically.

## 2. Mint a PAT for cross-repo formula bumps

The default `GITHUB_TOKEN` cannot push to a different repo. Mint a Personal Access Token:

- Fine-grained token recommended
  - Resource owner: `aerospike-ce-ecosystem`
  - Repository access: only `aerospike-ce-ecosystem/homebrew-tap`
  - Permissions: `Contents: read & write`, `Metadata: read`
- Or a classic PAT with `repo` scope (broader, but works everywhere)

Then in `aerospike-ce-ecosystem/ackoctl` → Settings → Secrets and variables → Actions → **New repository secret**:

| Name | Value |
|------|-------|
| `GH_AW_GITHUB_TOKEN` | the PAT |

## 3. Test with a snapshot tag

Push a candidate tag and watch the workflow:

```bash
git tag v0.0.0-test
git push origin v0.0.0-test
```

Expected:

- `release` workflow: green; the GitHub Release for `v0.0.0-test` lists the per-OS/arch `.tar.gz` archives, `checksums.txt`, and `install.sh`.
- `aerospike-ce-ecosystem/homebrew-tap` gets a `chore(formula): bump ackoctl to v0.0.0-test` commit on `main`.

Sanity-check from a clean machine (or container):

```bash
# Shell one-liner (Linux container)
podman run --rm ubuntu:24.04 bash -c '
  apt-get update -qq && apt-get install -y curl ca-certificates
  curl -fsSL https://raw.githubusercontent.com/aerospike-ce-ecosystem/ackoctl/main/install.sh | sh
  ackoctl version
'

# Homebrew (macOS host)
brew install aerospike-ce-ecosystem/tap/ackoctl
ackoctl version
```

Then delete the test release + tag if you don't want it lingering:

```bash
gh release delete v0.0.0-test --yes
git push origin :refs/tags/v0.0.0-test
git tag -d v0.0.0-test
```

## 4. Cutting real releases

```bash
git checkout main && git pull
git tag v0.2.0
git push origin v0.2.0
```

Done. The workflow runs automatically and within ~3 minutes:

- `brew upgrade ackoctl` picks up the new formula
- existing installs see `warning: ackoctl ... is outdated` on next invocation
- existing installs can self-update via `ackoctl upgrade`

## Troubleshooting

- **`release` succeeds but no formula bump** → `GH_AW_GITHUB_TOKEN` is unset or lacks `Contents: write` on the tap. Goreleaser silently skips the brew step in that case.
- **Release page missing `install.sh`** → check the `release.extra_files` glob in `.goreleaser.yaml`; the workflow uploads `install.sh` from the tagged tree so a refactor on `main` does not retroactively break older installer pipes.
