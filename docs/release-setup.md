# Release infrastructure setup

The `release.yml` + `publish-packages.yml` workflows are fully automated. The initial scaffolding (tap repo, gh-pages, GPG signing key, secrets) is already in place — this document records the current state and the few operator-facing knobs that remain.

Every `git push --tags` will:

1. Build binaries + `.tar.gz` + `.deb` + `.rpm` + `.apk` via goreleaser
2. Bump the Homebrew formula in `aerospike-ce-ecosystem/homebrew-tap`
3. Republish the APT + YUM repositories on `gh-pages`

…with no manual steps.

---

## Current state

| Resource | Status |
|----------|--------|
| `aerospike-ce-ecosystem/homebrew-tap` repository | created (public, `main` branch, empty `Formula/`) |
| `aerospike-ce-ecosystem/ackoctl` `gh-pages` branch | bootstrapped with placeholder `index.html` |
| GitHub Pages | enabled, serving from `gh-pages` / root at `https://aerospike-ce-ecosystem.github.io/ackoctl/` |
| `GH_AW_GITHUB_TOKEN` (org/repo secret, used by goreleaser to push the formula across repos) | present |
| `GPG_PRIVATE_KEY` (repo secret) | present — 4096-bit RSA, signs APT/YUM metadata + individual `.rpm` packages |
| `GPG_PASSPHRASE` (repo secret) | present |

The signing key fingerprint can be checked in `gh-pages` after the first release via `curl -fsSL https://aerospike-ce-ecosystem.github.io/ackoctl/key.gpg | gpg --show-keys`.

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

## 3. Generate the package-signing GPG key

This single key signs:
- APT `Release` → `Release.gpg` + `InRelease`
- YUM `repomd.xml` → `repomd.xml.asc`
- Every individual `.rpm` package

Generate locally (your laptop, not CI):

```bash
gpg --batch --gen-key <<EOF
%no-protection
Key-Type: RSA
Key-Length: 4096
Name-Real: aerospike-ce-ecosystem
Name-Email: release@aerospike-ce-ecosystem.example
Expire-Date: 5y
EOF
```

Drop `%no-protection` and add `Passphrase: …` if you prefer a passphrase-protected key (the workflow supports both via `GPG_PASSPHRASE`).

Export and grab the fingerprint:

```bash
KEY_ID=$(gpg --list-secret-keys --with-colons | awk -F: '/^sec/ {print $5; exit}')
gpg --armor --export-secret-keys "$KEY_ID" > ackoctl-signing.asc
gpg --armor --export             "$KEY_ID" > ackoctl-signing.pub
echo "fingerprint: $KEY_ID"
```

> **Back up `ackoctl-signing.asc` somewhere safe.** Lose the key and users can never trust newer signatures — they'll have to rotate to a new key on every machine.

Add the secrets to the ackoctl repo:

| Name | Value |
|------|-------|
| `GPG_PRIVATE_KEY` | the full contents of `ackoctl-signing.asc` (ASCII-armored) |
| `GPG_PASSPHRASE`  | the passphrase, or empty if you used `%no-protection` |

## 4. Bootstrap `gh-pages`

The first run of `publish-packages.yml` can create the branch itself, but it's tidier to seed it:

```bash
git checkout --orphan gh-pages
git rm -rf .
cat > index.html <<'EOF'
<!doctype html><meta charset="utf-8">
<title>ackoctl packages</title>
<h1>ackoctl package repositories</h1>
<p>This branch is automatically published by .github/workflows/publish-packages.yml.</p>
EOF
git add index.html
git commit -m "bootstrap gh-pages"
git push origin gh-pages
git checkout main
```

Then in the repo: **Settings → Pages → Build and deployment → Source = Deploy from a branch → Branch = `gh-pages` / `/ (root)`** → Save.

GitHub will assign `https://aerospike-ce-ecosystem.github.io/ackoctl/` — that's the base URL baked into the workflow and into all the install snippets in `README.md` / `docs/install.md`.

## 5. Test with a snapshot tag

Push a candidate tag and watch both workflows:

```bash
git tag v0.0.0-test
git push origin v0.0.0-test
```

Expected:

- `release` workflow: green; the GitHub Release for `v0.0.0-test` lists `.tar.gz`, `.deb`, `.rpm`, `.apk`, `checksums.txt`, `install.sh`.
- `publish-packages` workflow: green; `gh-pages` gets a new commit; `https://aerospike-ce-ecosystem.github.io/ackoctl/apt/dists/stable/InRelease` returns 200 with a clearsigned body.
- `aerospike-ce-ecosystem/homebrew-tap` gets a `chore(formula): bump ackoctl to v0.0.0-test` commit on `main`.

Sanity-check from a clean machine (or container):

```bash
# Homebrew
brew install aerospike-ce-ecosystem/tap/ackoctl
ackoctl version

# APT (Ubuntu container)
podman run --rm ubuntu:24.04 bash -c '
  apt-get update -qq && apt-get install -y curl gnupg ca-certificates
  install -d /etc/apt/keyrings
  curl -fsSL https://aerospike-ce-ecosystem.github.io/ackoctl/key.gpg \
    | gpg --dearmor -o /etc/apt/keyrings/ackoctl.gpg
  echo "deb [signed-by=/etc/apt/keyrings/ackoctl.gpg] https://aerospike-ce-ecosystem.github.io/ackoctl/apt stable main" \
    > /etc/apt/sources.list.d/ackoctl.list
  apt-get update && apt-get install -y ackoctl && ackoctl version
'

# DNF (Fedora container)
podman run --rm fedora:latest bash -c '
  curl -fsSL https://aerospike-ce-ecosystem.github.io/ackoctl/yum/ackoctl.repo \
    -o /etc/yum.repos.d/ackoctl.repo
  dnf install -y ackoctl && ackoctl version
'
```

Then delete the test release + tag if you don't want it lingering:

```bash
gh release delete v0.0.0-test --yes
git push origin :refs/tags/v0.0.0-test
git tag -d v0.0.0-test
```

(The `.deb`/`.rpm` for that test version will remain in the `gh-pages` pool/ until manually removed, which is fine — they just won't be advertised once you publish a newer real release.)

## 6. Cutting real releases

```bash
git checkout main && git pull
git tag v0.2.0
git push origin v0.2.0
```

Done. Both workflows run automatically and within ~5 minutes:

- `brew upgrade ackoctl`
- `apt update && apt install --only-upgrade ackoctl`
- `dnf upgrade ackoctl`

…will all pick up the new version.

## Key rotation

If the signing key is ever compromised or expires:

1. Generate a new key (Step 3).
2. Update `GPG_PRIVATE_KEY` / `GPG_PASSPHRASE` secrets.
3. Re-run `publish-packages` (Actions → Run workflow → enter the latest tag) to re-sign everything with the new key.
4. Users have to re-import `key.gpg` — communicate via release notes.

## Troubleshooting

- **`release` succeeds but no formula PR** → `GH_AW_GITHUB_TOKEN` is unset or lacks `Contents: write` on the tap. Goreleaser silently skips the brew step in that case.
- **`release` succeeds but `publish-packages` did not fire** → `GH_AW_GITHUB_TOKEN` is unset or lacks `Actions: write` on this repo, so the explicit `gh workflow run publish-packages.yml` step at the end of `release.yml` could not dispatch. Re-run by hand from the Actions UI (`Run workflow` → enter the tag) until the secret is fixed.
- **`publish-packages` fails at "Sign APT Release"** → the GPG key was imported but `GPG_PASSPHRASE` is wrong (or the key is passphrase-less and `GPG_PASSPHRASE` is set to garbage).
- **`apt update` says "NO_PUBKEY"** → the user is on an old `key.gpg`; have them re-download it.
- **`dnf` says "Signature verification failed"** → individual `.rpm` is unsigned. Probably an old release published before the rpmsign step was added; re-run `publish-packages` against that tag.
