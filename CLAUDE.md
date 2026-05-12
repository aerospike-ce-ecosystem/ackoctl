# ackoctl — AI Development Guide

A Go CLI for [aerospike-cluster-manager](../aerospike-cluster-manager/). Calls the FastAPI REST surface at `/api/v1/*`; does **not** talk to Kubernetes or Aerospike directly.

## Layout

```
cmd/ackoctl/         entry point (main.go)
internal/cli/        cobra commands (one file per noun)
internal/client/     REST client for cluster-manager
internal/config/     ~/.ackoctl/config.yaml parser (kubeconfig style)
internal/output/     -o table|json|yaml formatter
test/e2e/            kind-based smoke tests + openapi drift check
```

## Conventions

- **Command grammar**: `ackoctl <noun> <verb>` (gh/aws style). Example: `ackoctl connection list`, not `ackoctl get connections`.
- **Output**: default `table`, opt-in `-o json` / `-o yaml`. Always supply machine-readable output for any list/get command.
- **Errors**: parse cluster-manager FastAPI `{"detail": ...}` and surface via `APIError`. Exit code 1 on failure.
- **Workspace ACL**: every resource command honors `--workspace`; default comes from current context. Never silently fall back to "first workspace".
- **Auth**: bearer token only. No interactive `ackoctl login` — users bring their own OIDC JWT.

## Adding a new endpoint

1. Confirm shape against `/api/openapi.json` from a running cluster-manager (kind + `kubectl port-forward`).
2. Add request/response types to `internal/client/types.go` (mirror Pydantic models but only fields ackoctl uses).
3. Add a method on `BaseClient` in the matching `internal/client/<noun>s.go` file.
4. Add the cobra command in `internal/cli/<noun>.go`.
5. Extend `test/e2e/openapi_test.go` to assert the endpoint still exists in the live spec.

## Test layers

- **Unit**: `go test ./internal/...` — fast, hermetic, runs in CI.
- **E2E**: `make test-e2e` — requires kind + cluster-manager up, drives real CLI invocations.

## Versioning & release

- Version/commit/build-date are injected via `-ldflags` from `Makefile` / goreleaser.
- Conventional Commits (`feat`, `fix`, `chore`, `docs`, `refactor`, `test`).
- Apache-2.0.
