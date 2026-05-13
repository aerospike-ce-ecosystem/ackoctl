# In-cluster E2E — kind + ACKO + cluster-manager

Manual scenario that proves the upstream `install.sh` one-liner produces a working `ackoctl` binary that can drive ACKO **from inside the same Kubernetes cluster** — the realistic path for CI runners, jump pods, and operations workflows.

## What this verifies

```
ubuntu/alpine pod  (ackoctl installed via install.sh)
   │
   │  HTTP /api/v1/k8s/clusters
   ▼
acko-aerospike-ce-kubernetes-operator-ui-api  (ClusterIP service)
   │
   │  K8s API
   ▼
ACKO operator
   │
   │  reconciles
   ▼
AerospikeCluster CR
```

## Prerequisites

- `kind` cluster running (any recent version; tested on K8s 1.35)
- `kubectl`, `helm`, `podman` (or `docker`) on the host
- The ACKO helm chart 1.3.x with UI enabled:

  ```bash
  helm upgrade --install acko \
    oci://ghcr.io/aerospike-ce-ecosystem/charts/aerospike-ce-kubernetes-operator \
    --version 1.3.1 \
    --namespace aerospike-operator --create-namespace \
    -f values.yaml --wait
  ```

  with `values.yaml`:
  ```yaml
  crds:
    install: false   # if you already applied the CRDs chart separately
  defaultTemplates:
    enabled: false
  ui:
    enabled: true
    api:
      enabled: true
    web:
      enabled: true
  ```

  Verify:
  ```bash
  kubectl -n aerospike-operator get svc \
    acko-aerospike-ce-kubernetes-operator-ui-api
  ```

## Test pod

The script below runs inside the cluster. It installs `ackoctl` via the
upstream installer, points it at the in-cluster service over its DNS name,
and asks cluster-manager for the list of `AerospikeCluster` CRs ACKO is
reconciling.

`pod-install-sh-test.sh`:
```bash
#!/bin/bash
set -uo pipefail
apt-get update -qq
apt-get install -y --no-install-recommends curl ca-certificates >/dev/null

curl -fsSL https://raw.githubusercontent.com/aerospike-ce-ecosystem/ackoctl/main/install.sh \
  | sh

API=http://acko-aerospike-ce-kubernetes-operator-ui-api.aerospike-operator.svc.cluster.local/api
ackoctl config set-context kind --server="$API" --workspace-id=default
ackoctl config use-context kind
ackoctl k8s cluster list
```

Run it:
```bash
kubectl create configmap ackoctl-install-test --from-file=run.sh=pod-install-sh-test.sh
kubectl run ackoctl-install --restart=Never --image=ubuntu:24.04 \
  --overrides='{"spec":{"containers":[{"name":"ackoctl-install","image":"ubuntu:24.04","command":["bash","/scripts/run.sh"],"volumeMounts":[{"name":"scripts","mountPath":"/scripts"}]}],"volumes":[{"name":"scripts","configMap":{"name":"ackoctl-install-test","defaultMode":493}}]}}'
kubectl wait --for=condition=Ready pod/ackoctl-install --timeout=5m || true
kubectl logs ackoctl-install
```

Expected output excerpt:
```
ackoctl version 0.1.3
  os/arch: linux/arm64
NAMESPACE  NAME         PHASE      NODES
aerospike  testcluster  Completed  1
```

To exercise the alpine path, swap the image to `alpine:3` and replace the
apt prelude with `apk add --no-cache curl bash ca-certificates` — `install.sh`
is POSIX-shell only and runs unmodified.

## Cleanup

```bash
kubectl delete pod ackoctl-install --ignore-not-found
kubectl delete configmap ackoctl-install-test --ignore-not-found
```

## Notes

- The in-cluster DNS name above (`acko-aerospike-ce-kubernetes-operator-ui-api.aerospike-operator.svc.cluster.local`) is the default for a release named `acko` in namespace `aerospike-operator`. If your release/namespace differs, run `kubectl get svc -A | grep ui-api` to find the actual name.
- `ackoctl config set-context` only configures the client; no in-cluster RBAC binding is needed because cluster-manager itself owns the Kubernetes API access. Auth (`--token` / `ACKOCTL_TOKEN`) is the cluster-manager bearer, not a K8s ServiceAccount token.
- For namespaced commands you may need `--namespace`/`-N` rather than `-n` depending on the verb — see `ackoctl k8s cluster <verb> --help`.
- The pod above does not need internet egress for `apt update` itself, only for `raw.githubusercontent.com` (install.sh) and `github.com` (release tarball). In air-gapped clusters mirror those two URLs into your internal registry/proxy.
- Tested with: ACKO chart 1.3.1, K8s 1.35 (kind), ackoctl v0.1.3, arm64 host.
