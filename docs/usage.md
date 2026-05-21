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

`--bins` takes the **entire bin set as a single JSON object** — not repeatable key/value pairs. `--bins='{"name":"Alice","age":30}'` is correct; `--bins=name=Alice --bins=age=30` returns `--bins must be a JSON object`. JSON typing is preserved end-to-end, so numbers stay numbers and quoted strings stay strings.

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

## info — asinfo passthrough

```bash
# Fan-out across every reachable node
ackoctl info <CONN_ID> --command=build --command=status

# Target a single node
ackoctl info <CONN_ID> --command=statistics --node=BB9020011AC4202

# Forward a write verb (off the read-only whitelist)
ackoctl info <CONN_ID> --allow-write --command='set-config:context=service;proto-fd-max=20000'
```

cluster-manager enforces a read-only whitelist by default (`build`, `status`, `statistics`, `namespaces`, `namespace/<ns>`, ...). `--allow-write` bypasses the whitelist so verbs such as `set-config:` go through. One row per `(node, command)` pair is returned.

---

## admin — Aerospike security users and roles

Requires `security { enable-security true }` on the target cluster. **Aerospike Community Edition does not ship the security module**, so every admin call fails on a CE cluster — this group is here for Enterprise targets that cluster-manager happens to know about.

```bash
# Users
ackoctl admin user list   <CONN_ID>
ackoctl admin user create <CONN_ID> --username=alice --password-stdin --roles=read,write <<<'s3cret'
ackoctl admin user passwd <CONN_ID> --username=alice --password-stdin <<<'new-s3cret'
ackoctl admin user delete <CONN_ID> --username=alice --yes

# Roles
ackoctl admin role list   <CONN_ID>
ackoctl admin role create <CONN_ID> --name=analyst --privilege=read:test --privilege=sindex-admin:test
ackoctl admin role delete <CONN_ID> --name=analyst --yes
```

Prefer `--password-stdin` over `--password` — the plaintext form ends up in shell history.

---

## note — operator memos stored in cluster-manager

Notes are free-text annotations stored in cluster-manager's metaDB (not in Aerospike itself). They are scoped per connection profile and cascade-delete with the connection. Use them for runbook context, ticket references, or known-issue markers.

```bash
# Set-level notes
ackoctl note set list   <CONN_ID>
ackoctl note set update <CONN_ID> --namespace=test --set=users --note='Migrated from legacy cluster on 2026-01-15'
ackoctl note set delete <CONN_ID> --namespace=test --set=users --yes

# Record-level notes (note body up to 8 KB)
ackoctl note record list   <CONN_ID> --namespace=test --set=users
ackoctl note record update <CONN_ID> --namespace=test --set=users --pk=alice --note='VIP — see ticket OPS-1234'
ackoctl note record delete <CONN_ID> --namespace=test --set=users --pk=alice --yes
```

---

## guide — operational guides (org/team policy)

Guides are workspace-scoped Markdown policy documents managed in cluster-manager. Each workspace has a **data-plane** guide (policy for Aerospike data CRUD) and a **control-plane** guide (policy for cluster lifecycle). Read the relevant guide **before** running data or cluster operations so your changes follow the org/team policy. This command is read-only — guides are authored by acko administrators in the cluster-manager web UI.

The workspace comes from `--workspace` or the current context; when neither is set it falls back to the built-in `ws-default` workspace.

```bash
ackoctl guide list                              # both guides registered for the workspace
ackoctl guide get data-plane                    # prints the Markdown body (read before record/set/query writes)
ackoctl guide get control-plane                 # read before creating/scaling/deleting clusters
ackoctl guide get data-plane --workspace=ws-team-a
ackoctl guide get control-plane -o json         # structured: title, timestamps, author
```

`guide get` prints the raw Markdown to stdout under the default output so it reads naturally and pipes cleanly; `-o json` / `-o yaml` emit the full structured guide.

---

## udf — Lua user-defined functions

Only Lua is supported on Aerospike CE. Requests pass through cluster-manager's `/api/v1/udfs` surface (single JSON `{"filename":..., "content":<source>}` body, not multipart).

```bash
ackoctl udf list   <CONN_ID>
ackoctl udf upload <CONN_ID> --file=./helpers.lua                 # filename defaults to basename
ackoctl udf upload <CONN_ID> --file=./helpers.lua --filename=my_module.lua
ackoctl udf remove <CONN_ID> --filename=my_module.lua --yes
```

cluster-manager validates `--filename` against `^[a-zA-Z0-9_.-]{1,255}$`; invalid names come back as HTTP 422.

---

## Output formats

`-o json` and `-o yaml` always honor the cluster-manager schema verbatim — feed them to `jq` / `yq` / scripts.

`-o table` is the default and is best-effort:

- list commands have hand-tuned columns,
- single-resource commands (get, info, health) fall back to a key/value tree.

If a future schema change misaligns the table view, prefer `-o yaml` until ackoctl is updated.
