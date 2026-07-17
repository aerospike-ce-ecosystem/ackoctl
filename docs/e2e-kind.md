# In-cluster E2E — kind + ACKO + cluster-manager

This manual scenario verifies that the upstream `install.sh` script produces a working `ackoctl` binary. It runs `ackoctl` **inside the same Kubernetes cluster** as ACKO, which matches common CI runner, jump pod, and operations workflows.

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

The script runs inside the cluster. It installs `ackoctl` with the upstream installer and connects to the in-cluster service through its DNS name. It then asks Cluster Manager for the `AerospikeCluster` CRs that ACKO is reconciling.

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
ackoctl version 0.2.0
  os/arch: linux/arm64
NAMESPACE  NAME         PHASE      NODES
aerospike  testcluster  Completed  1
```

To test Alpine, change the image to `alpine:3` and replace the apt commands with `apk add --no-cache curl bash ca-certificates`. The POSIX `install.sh` script runs without other changes.

## Cleanup

```bash
kubectl delete pod ackoctl-install --ignore-not-found
kubectl delete configmap ackoctl-install-test --ignore-not-found
```

## Notes

- The in-cluster DNS name above (`acko-aerospike-ce-kubernetes-operator-ui-api.aerospike-operator.svc.cluster.local`) assumes a release named `acko` in the `aerospike-operator` namespace. For another release or namespace, run `kubectl get svc -A | grep ui-api` to find the service name.
- `ackoctl config set-context` configures only the client. You do not need an in-cluster RBAC binding because Cluster Manager owns Kubernetes API access. Authentication through `--token` or `ACKOCTL_TOKEN` uses the Cluster Manager bearer token, not a K8s ServiceAccount token.
- For namespaced commands you may need `--namespace`/`-N` rather than `-n` depending on the verb — see `ackoctl k8s cluster <verb> --help`.
- The pod needs internet egress for `raw.githubusercontent.com` (`install.sh`) and `github.com` (the release archive). For air-gapped clusters, mirror both URLs through an internal registry or proxy.
- Tested with: ACKO chart 1.3.1, K8s 1.35 (kind), ackoctl v0.2.0, arm64 host.
