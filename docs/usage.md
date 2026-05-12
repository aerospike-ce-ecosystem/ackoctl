# ackoctl usage

`ackoctl` is a kubectl/gh-style CLI for [aerospike-cluster-manager](https://github.com/aerospike-ce-ecosystem/aerospike-cluster-manager). All commands hit cluster-manager's REST API at `/api/v1/*`.

## Global flags

| Flag | Description |
|------|-------------|
| `--config PATH` | Override config file location (default `~/.ackoctl/config.yaml`, also reads `$ACKOCTL_CONFIG`). |
| `--context NAME` | Use a specific context instead of `current-context`. |
| `--server URL` | One-off server override (e.g. `http://localhost:8000/api`). |
| `--token TOKEN` | One-off bearer token. Obtain via your IdP — `ackoctl` has no `login`. |
| `--workspace ID` | cluster-manager workspace id for ACL scoping. |
| `-o table\|json\|yaml` | Output format (default `table`). |
| `--insecure-skip-tls` | Skip TLS verification (dev only). |
| `-v, --verbose` | Verbose logging to stderr. |

Override order: **CLI flag > environment variable > config file**.

Environment overrides: `ACKOCTL_CONFIG`, `ACKOCTL_CONTEXT`, `ACKOCTL_SERVER`, `ACKOCTL_TOKEN`, `ACKOCTL_WORKSPACE`.

---

## config — context management

```bash
ackoctl config set-context kind-local \
  --server=http://localhost:8000/api \
  --workspace-id=default
ackoctl config set-context prod \
  --server=https://acm.example.com/api --token=eyJ...
ackoctl config use-context prod
ackoctl config current-context
ackoctl config view -o yaml
ackoctl config delete-context prod
```

---

## connection — Aerospike connection profiles

```bash
# Discover what's registered
ackoctl connection list

# Add a new connection (--host repeats for multi-node seeds)
ackoctl connection create \
  --name local-aero \
  --host aerospike-node-1 --host aerospike-node-2 \
  --port 3000 \
  --label env=dev --label team=platform

# Inspect / patch / remove
ackoctl connection get  <ID>
ackoctl connection update <ID> --name renamed
ackoctl connection delete <ID> --yes

# Live probe (always returns 200 — see `connected` field)
ackoctl connection health <ID>
```

---

## cluster — Aerospike cluster inspection

```bash
# Full cluster snapshot (nodes, namespaces, sets, sindex counts)
ackoctl cluster info <CONN_ID> -o yaml

# Tune runtime-mutable namespace knobs (asinfo set-config under the hood).
# Aerospike CE does NOT support creating namespaces at runtime — they live
# in aerospike.conf.
ackoctl cluster configure-namespace <CONN_ID> \
  --name=test \
  --param=high-water-disk-pct=70 \
  --param=stop-writes-pct=90
```

---

## k8s — ACKO-managed Kubernetes clusters

Requires cluster-manager to have `K8S_MANAGEMENT_ENABLED=true`. Otherwise the server returns 404.

```bash
ackoctl k8s cluster list                                 # all AerospikeCluster CRs
ackoctl k8s cluster get aerospike/sample-cluster
ackoctl k8s cluster reconcile aerospike/sample-cluster   # stamp acko.io/force-reconcile
```

---

## record — data plane

```bash
# List with paging
ackoctl record list <CONN_ID> --namespace=test --set=users --page-size=100

# Read / write / delete a single record
ackoctl record get <CONN_ID> --namespace=test --set=users --pk=alice
ackoctl record put <CONN_ID> --namespace=test --set=users --pk=alice \
  --bins='{"name":"Alice","age":30}' --ttl=3600
ackoctl record delete <CONN_ID> --namespace=test --set=users --pk=alice --yes

# Filtered scan — pk-pattern, predicate filters, expression all supported
ackoctl record query <CONN_ID> \
  --namespace=test --set=users \
  --pk-pattern='ali' --pk-match-mode=prefix \
  --select=name,age --page-size=50
```

`--filter` and `--predicate` accept raw JSON to pass through cluster-manager's `FilterGroup` / `QueryPredicate` DSL when you need the full power.

`--pk-type` lets you pin the particle type (`auto|string|int|bytes`). With `auto` cluster-manager will retry the alternate type on `NOT_FOUND`.

---

## set — derived set inventory

```bash
ackoctl set list <CONN_ID>                       # all namespaces
ackoctl set list <CONN_ID> --namespace=test      # one namespace
```

There's no dedicated `/sets` endpoint on the server; ackoctl pulls the cluster info response and extracts `namespaces[].sets[]`.

---

## query — predicate / pk-lookup / full scan

```bash
# Predicate: --value/--value2 parse as JSON (so 30 stays int, "alice" stays string)
ackoctl query exec <CONN_ID> --namespace=test --set=users \
  --bin=age --op=between --value=18 --value2=30 --select=name,age

# Primary-key lookup
ackoctl query exec <CONN_ID> --namespace=test --set=users \
  --primary-key=alice --pk-type=string

# Full scan capped at 1000 records
ackoctl query exec <CONN_ID> --namespace=test --set=users --max-records=1000
```

Operators: `equals | between | contains | geo_within_region | geo_contains_point`.

---

## index — secondary indexes

```bash
ackoctl index list   <CONN_ID>
ackoctl index create <CONN_ID> \
  --namespace=test --set=users \
  --bin=age --name=idx_age --type=numeric
ackoctl index delete <CONN_ID> --namespace=test --name=idx_age --yes
```

`--type` is one of `numeric | string | geo2dsphere`.

---

## Output formats

`-o json` and `-o yaml` always honor the cluster-manager schema verbatim — feed them to `jq` / `yq` / scripts.

`-o table` is the default and is best-effort:

- list commands have hand-tuned columns,
- single-resource commands (get, info, health) fall back to a key/value tree.

If a future schema change misaligns the table view, prefer `-o yaml` until ackoctl is updated.
