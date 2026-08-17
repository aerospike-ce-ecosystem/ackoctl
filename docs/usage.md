# ackoctl usage

`ackoctl` follows the command style of kubectl and gh. It sends every command to [aerospike-cluster-manager](https://github.com/aerospike-ce-ecosystem/aerospike-cluster-manager) through the `/api/v1/*` REST API.

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

Environment overrides: `ACKOCTL_CONFIG`, `ACKOCTL_CONTEXT`, `ACKOCTL_SERVER`, `ACKOCTL_TOKEN`, `ACKOCTL_WORKSPACE`, `ACKOCTL_INSECURE_SKIP_TLS`.

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
#
# Only memorySize and replicationFactor are accepted. Read the section below
# before changing just one of them.
ackoctl cluster configure-namespace <CONN_ID> \
  --name=test \
  --param=memorySize=2147483648 \
  --param=replicationFactor=2
```

### configure-namespace reads only two knobs

Cluster Manager's `CreateNamespaceRequest` declares exactly three fields — `name`, `memorySize` (bytes), and `replicationFactor` — and ignores every other key in the body. It then applies the numeric fields to the running namespace via a single `set-config`:

```
set-config:context=namespace;id=<ns>;memory-size=<memorySize>;replication-factor=<replicationFactor>
```

So:

- **Any other `--param` key is rejected client-side.** The server would drop it silently and still return 200, so `ackoctl` refuses rather than let you believe a setting was applied. Use the asinfo passthrough for those (below).
- **`ackoctl` sends only the knobs you supply.** You can change the replication factor without restating the memory size.

#### Changing only one knob

A Cluster Manager deployment that predates [aerospike-cluster-manager#478](https://github.com/aerospike-ce-ecosystem/aerospike-cluster-manager/pull/478) substitutes its **own default** for whichever knob you omit — `memorySize=1073741824` (1 GiB) or `replicationFactor=2` — and applies it to the live namespace. Changing one knob on such a server can therefore shrink a larger namespace to 1 GiB or reset its replication factor as a side effect, risking eviction or stop-writes.

`ackoctl` gates the one-knob form on `--yes/-y` and names the field at risk:

```console
$ ackoctl cluster configure-namespace <CONN_ID> --name=test --param=replicationFactor=3
Error: confirmation required (--yes): memorySize was not supplied (server default: 1073741824 bytes). A cluster-manager without the fix in https://github.com/aerospike-ce-ecosystem/aerospike-cluster-manager/pull/478 substitutes its own default and applies it to the running namespace "test". Supply the value explicitly, or pass --yes to accept whatever the server does with it
```

Two safe ways forward. Either read the current value and pass both knobs:

```bash
ackoctl info <CONN_ID> --command='namespace/test' | tr ';' '\n' | grep -E 'memory-size|replication-factor'

ackoctl cluster configure-namespace <CONN_ID> --name=test \
  --param=memorySize=<current value> \
  --param=replicationFactor=3
```

Or, on a server that already carries the fix, confirm the one-knob change:

```bash
ackoctl cluster configure-namespace <CONN_ID> --name=test --param=replicationFactor=3 --yes
```

The confirmation and its warning exist only for pre-#478 servers; once every deployment you talk to carries that fix, the one-knob form is safe on its own.

For any knob outside those two, use the asinfo passthrough:

```bash
ackoctl info <CONN_ID> --allow-write \
  --command='set-config:context=namespace;id=test;high-water-disk-pct=70'
```

---

## k8s — ACKO-managed Kubernetes clusters

Set `K8S_MANAGEMENT_ENABLED=true` in Cluster Manager before you use these commands. Otherwise, the server returns 404.

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

# Remove a single bin from a record (leaves the rest of the record intact)
ackoctl record delete-bin <CONN_ID> --namespace=test --set=users --pk=alice --bin=temp --yes

# Filtered scan — pk-pattern, predicate filters, expression all supported
ackoctl record query <CONN_ID> \
  --namespace=test --set=users \
  --pk-pattern='ali' --pk-match-mode=prefix \
  --select=name,age --page-size=50
```

`--bins` accepts the **complete bin set as one JSON object**, not repeated key/value pairs. Use `--bins='{"name":"Alice","age":30}'`. The form `--bins=name=Alice --bins=age=30` returns `--bins must be a JSON object`. JSON types remain intact, so numbers stay numbers and quoted strings stay strings.

Use `--filter` and `--predicate` to pass raw JSON to Cluster Manager's `FilterGroup` / `QueryPredicate` DSL.

Use `--pk-type` to set the particle type (`auto|string|int|bytes`). With `auto`, Cluster Manager retries the alternate type after `NOT_FOUND`.

---

## set — derived set inventory and truncate

```bash
ackoctl set list <CONN_ID>                       # all namespaces
ackoctl set list <CONN_ID> --namespace=test      # one namespace

# Wipe every record in a set. Destructive: --yes/-y is mandatory.
ackoctl set truncate <CONN_ID> --namespace=test --set=users --yes

# Truncate only records last updated before a nanosecond cutoff
# (since the CITRUS epoch, 2010-01-01 UTC)
ackoctl set truncate <CONN_ID> --namespace=test --set=users \
  --before-lut=473342400000000000 --yes
```

`set list` has no dedicated server endpoint. `ackoctl` reads the cluster information response and extracts `namespaces[].sets[]`.

`set truncate` removes records permanently — there is no undo, and Aerospike applies it asynchronously across the cluster. Notes:

- `--yes/-y` is required. `ackoctl` has no interactive prompt, so this is the only confirmation.
- Omit `--before-lut` to wipe the whole set. When given, only records whose last-update-time is **below** the cutoff are truncated.
- `--before-lut=0` is rejected client-side and server-side: at the asinfo layer `lut=0` means "truncate everything", so omitting the flag is the explicit way to ask for that.
- Cluster Manager rate-limits this endpoint to **10 requests per minute**; beyond that you get HTTP 429 (exit code 4).

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

By default, Cluster Manager allows only read-only commands such as `build`, `status`, `statistics`, `namespaces`, and `namespace/<ns>`. `--allow-write` bypasses this list and permits commands such as `set-config:`. The response contains one row for each `(node, command)` pair.

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

Use `--password-stdin` instead of `--password` so the plaintext password does not remain in shell history.

---

## note — operator memos stored in cluster-manager

Notes are free-text annotations in Cluster Manager's metaDB, not in Aerospike. Each note belongs to a connection profile and is deleted with that connection. Use notes for runbook context, ticket references, or known issues.

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

Guides are workspace-scoped Markdown policy documents stored in Cluster Manager. Each workspace has a **data-plane** guide for Aerospike data CRUD and a **control-plane** guide for cluster lifecycle operations. Read the relevant guide **before** you change data or clusters. The command is read-only; acko administrators author guides in the Cluster Manager web UI.

The workspace comes from `--workspace` or the current context; when neither is set it falls back to the built-in `ws-default` workspace.

```bash
ackoctl guide list                              # both guides registered for the workspace
ackoctl guide get data-plane                    # prints the Markdown body (read before record/set/query writes)
ackoctl guide get control-plane                 # read before creating/scaling/deleting clusters
ackoctl guide get data-plane --workspace=ws-team-a
ackoctl guide get control-plane -o json         # structured: title, timestamps, author
```

With the default output, `guide get` prints raw Markdown to stdout for easy reading and piping. Use `-o json` or `-o yaml` for the complete structured guide.

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

`-o json` and `-o yaml` preserve the Cluster Manager schema. You can pass their output to `jq`, `yq`, or scripts.

`-o table` is the default and uses a best-effort layout:

- list commands have hand-tuned columns,
- single-resource commands (get, info, health) fall back to a key/value tree.

If a future schema change misaligns the table view, prefer `-o yaml` until ackoctl is updated.

---

## Exit codes

`ackoctl` returns structured exit codes so a script can tell a caller-side mistake from a server problem from a user abort, without parsing stderr.

| Code | Meaning | Retry? |
|------|---------|--------|
| `0` | Success. | — |
| `1` | Generic failure: bad flags, client-side validation, transport or parse error, missing/unusable config, or an HTTP status outside 400–599. | No — fix the invocation. |
| `4` | Cluster Manager returned **4xx** (bad request, auth, not found, 422 validation, 429 rate limit). | No — the request itself is wrong. |
| `5` | Cluster Manager returned **5xx** (upstream or transient server failure). | Yes — a retry may succeed. |
| `130` | Aborted by `SIGINT` (ctrl-c) or `SIGTERM`. Follows the shell's `128 + signal` convention. | N/A |

Every failure also writes one `Error: …` line to stderr; stdout carries only command output.

```bash
ackoctl record get "$CONN" --namespace=test --set=users --pk=alice -o json
case $? in
  0)   ;;                                       # got the record
  4)   echo "bad request or missing record" >&2; exit 1 ;;
  5)   sleep 5; exec "$0" "$@" ;;               # transient — retry
  130) echo "aborted" >&2; exit 130 ;;
  *)   echo "ackoctl failed" >&2; exit 1 ;;
esac
```

A config error (no current context, unknown context) exits `1` and adds a `hint:` line pointing at `ackoctl config set-context`.
