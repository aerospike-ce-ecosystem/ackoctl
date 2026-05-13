# In-cluster E2E — kind + ACKO + cluster-manager

Manual scenario that proves `apt install ackoctl` and `dnf install ackoctl` produce a working binary that can drive ACKO **from inside the same Kubernetes cluster** — the realistic path for CI runners, jump pods, and operations workflows.

## What this verifies

```
ubuntu/fedora pod (ackoctl installed via apt or dnf)
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

## Test pods

The scripts below are run inside the cluster. They install `ackoctl` from the official apt / yum repos, point it at the in-cluster service over its DNS name, and ask cluster-manager for the list of `AerospikeCluster` CRs ACKO is reconciling.

### apt path

`pod-apt-test.sh`:
```bash
#!/bin/bash
set -uo pipefail
apt-get update -qq
apt-get install -y --no-install-recommends curl gnupg ca-certificates >/dev/null

install -d /etc/apt/keyrings
curl -fsSL https://aerospike-ce-ecosystem.github.io/ackoctl/key.gpg \
  | gpg --dearmor -o /etc/apt/keyrings/ackoctl.gpg
echo "deb [signed-by=/etc/apt/keyrings/ackoctl.gpg] https://aerospike-ce-ecosystem.github.io/ackoctl/apt stable main" \
  > /etc/apt/sources.list.d/ackoctl.list
apt-get update -qq
apt-get install -y --no-install-recommends ackoctl >/dev/null

API=http://acko-aerospike-ce-kubernetes-operator-ui-api.aerospike-operator.svc.cluster.local/api
ackoctl config set-context kind --server="$API" --workspace-id=default
ackoctl config use-context kind
ackoctl k8s cluster list
```

Run it:
```bash
kubectl create configmap ackoctl-apt-test --from-file=run.sh=pod-apt-test.sh
kubectl run ackoctl-apt --restart=Never --image=ubuntu:24.04 \
  --overrides='{"spec":{"containers":[{"name":"ackoctl-apt","image":"ubuntu:24.04","command":["bash","/scripts/run.sh"],"volumeMounts":[{"name":"scripts","mountPath":"/scripts"}]}],"volumes":[{"name":"scripts","configMap":{"name":"ackoctl-apt-test","defaultMode":493}}]}}'
kubectl wait --for=condition=Ready pod/ackoctl-apt --timeout=5m || true
kubectl logs ackoctl-apt
```

Expected output excerpt:
```
ackoctl version 0.1.3
  os/arch: linux/arm64
NAMESPACE  NAME         PHASE      NODES
aerospike  testcluster  Completed  1
```

### dnf path

`pod-dnf-test.sh`:
```bash
#!/bin/bash
set -uo pipefail
curl -fsSL https://aerospike-ce-ecosystem.github.io/ackoctl/yum/ackoctl.repo \
  -o /etc/yum.repos.d/ackoctl.repo
dnf --assumeyes --quiet install ackoctl >/dev/null

API=http://acko-aerospike-ce-kubernetes-operator-ui-api.aerospike-operator.svc.cluster.local/api
ackoctl config set-context kind --server="$API" --workspace-id=default
ackoctl config use-context kind
ackoctl k8s cluster list
```

Run it (substitute `fedora:latest` for the image and adjust pod name).

## Cleanup

```bash
kubectl delete pod ackoctl-apt ackoctl-dnf --ignore-not-found
kubectl delete configmap ackoctl-apt-test ackoctl-dnf-test --ignore-not-found
```

## Notes

- The in-cluster DNS name above (`acko-aerospike-ce-kubernetes-operator-ui-api.aerospike-operator.svc.cluster.local`) is the default for a release named `acko` in namespace `aerospike-operator`. If your release/namespace differs, run `kubectl get svc -A | grep ui-api` to find the actual name.
- `ackoctl config set-context` only configures the client; no in-cluster RBAC binding is needed because cluster-manager itself owns the Kubernetes API access. Auth (`--token` / `ACKOCTL_TOKEN`) is the cluster-manager bearer, not a K8s ServiceAccount token.
- For namespaced commands you may need `--namespace`/`-N` rather than `-n` depending on the verb — see `ackoctl k8s cluster <verb> --help`.
- Tested with: ACKO chart 1.3.1, K8s 1.35 (kind), ackoctl v0.1.3 (apt + dnf), arm64 host.
