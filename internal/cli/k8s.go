package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/client"
	"github.com/aerospike-ce-ecosystem/ackoctl/internal/output"
)

// missingFieldSentinel marks a table cell where the server response did not
// include the expected key. Distinguishing missing from empty helps users
// spot schema drift instead of assuming the resource has no value.
const missingFieldSentinel = "<missing>"

func newK8sCmd(global *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "k8s",
		Short: "Operate on ACKO-managed Kubernetes Aerospike clusters",
		Long: `Requires cluster-manager to have K8S_MANAGEMENT_ENABLED=true.
When disabled, every k8s subcommand returns HTTP 404 from the server.`,
	}
	cmd.AddCommand(newK8sClusterCmd(global))
	return cmd
}

func newK8sClusterCmd(global *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Manage AerospikeCluster CRs",
	}
	cmd.AddCommand(
		newK8sClusterListCmd(global),
		newK8sClusterGetCmd(global),
		newK8sClusterReconcileCmd(global),
		newK8sClusterScaleCmd(global),
		newK8sClusterEventsCmd(global),
		newK8sClusterLogsCmd(global),
		newK8sClusterPodsCmd(global),
	)
	return cmd
}

func newK8sClusterListCmd(global *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List ACKO-managed clusters",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			items, err := c.ListK8sClusters(cmd.Context())
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, items,
				output.WithTable(
					[]string{"NAMESPACE", "NAME", "PHASE", "NODES"},
					func(v any) []string {
						row := v.(client.K8sCluster)
						return []string{
							stringField(row, "namespace"),
							stringField(row, "name"),
							stringField(row, "phase"),
							stringField(row, "size"),
						}
					},
					func(any) []any {
						rows := make([]any, 0, len(items))
						for _, it := range items {
							rows = append(rows, it)
						}
						return rows
					},
				),
			)
		},
	}
}

func newK8sClusterGetCmd(global *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get NAMESPACE/NAME",
		Short: "Get a single ACKO-managed cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ns, name, err := splitNamespacedName(args[0])
			if err != nil {
				return err
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			cluster, err := c.GetK8sCluster(cmd.Context(), ns, name)
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, cluster)
		},
	}
}

func newK8sClusterReconcileCmd(global *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "reconcile NAMESPACE/NAME",
		Short: "Force the ACKO operator to re-reconcile this cluster",
		Long: `Adds the acko.io/force-reconcile annotation to the AerospikeCluster CR.
Useful when the cluster is stuck in a drifted state.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ns, name, err := splitNamespacedName(args[0])
			if err != nil {
				return err
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			out, err := c.ForceReconcileK8sCluster(cmd.Context(), ns, name)
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, out)
		},
	}
}

func newK8sClusterScaleCmd(global *GlobalFlags) *cobra.Command {
	var (
		size int
		yes  bool
	)
	cmd := &cobra.Command{
		Use:   "scale NAMESPACE/NAME --size N",
		Short: "Scale an ACKO-managed cluster to N nodes",
		Long: `Patches spec.size on the AerospikeCluster CR via
POST /k8s/clusters/{ns}/{name}/scale. CE caps the cluster at 8 nodes; both
the CLI and the server reject sizes outside 1..8.

Scale-down (target < current size) requires --yes/-y because shrinking
the cluster ejects nodes and can lose data on unreplicated partitions.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if size < 1 || size > 8 {
				return fmt.Errorf("--size must be between 1 and 8 (CE cap)")
			}
			ns, name, err := splitNamespacedName(args[0])
			if err != nil {
				return err
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			current, err := c.GetK8sCluster(cmd.Context(), ns, name)
			if err != nil {
				return err
			}
			cur, ok := intField(current, "size")
			switch {
			case !ok && !yes:
				// The GET response did not carry a resolvable node count, so we
				// cannot tell whether this is a scale-down. Treat "cannot
				// confirm" as "must confirm" and require --yes rather than
				// skipping the guard — silently proceeding could shrink the
				// cluster and lose data on unreplicated partitions.
				return fmt.Errorf("cannot determine current cluster size from the server response; pass --yes to proceed with scaling to %d", size)
			case ok && size < cur && !yes:
				return fmt.Errorf("refusing scale-down %d -> %d without --yes (data-loss risk on unreplicated partitions)", cur, size)
			}
			out, err := c.ScaleK8sCluster(cmd.Context(), ns, name, size)
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, out,
				output.WithTable(
					[]string{"NAMESPACE", "NAME", "PHASE", "SIZE"},
					func(v any) []string {
						row := v.(client.K8sCluster)
						return []string{
							stringField(row, "namespace"),
							stringField(row, "name"),
							stringField(row, "phase"),
							stringField(row, "size"),
						}
					},
					func(any) []any {
						return []any{out}
					},
				),
			)
		},
	}
	cmd.Flags().IntVar(&size, "size", 1, "target node count (1..8, CE cap)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "confirm scale-down (required when target < current size)")
	_ = cmd.MarkFlagRequired("size")
	return cmd
}

// intField extracts an integer from a map[string]any returned by the API.
// The REST client decodes raw-map responses with json.Number, so that is the
// common case; float64/int/int64 are still accepted for robustness against
// callers (and tests) that build the map directly.
func intField(m map[string]any, key string) (int, bool) {
	v, ok := m[key]
	if !ok || v == nil {
		return 0, false
	}
	switch t := v.(type) {
	case json.Number:
		n, err := t.Int64()
		if err != nil {
			return 0, false
		}
		return int(n), true
	case float64:
		return int(t), true
	case int:
		return t, true
	case int64:
		return int(t), true
	}
	return 0, false
}

func newK8sClusterEventsCmd(global *GlobalFlags) *cobra.Command {
	var (
		limit    int
		category string
		since    time.Duration
	)
	cmd := &cobra.Command{
		Use:   "events NAMESPACE/NAME",
		Short: "List Kubernetes events for an ACKO-managed cluster",
		Long: `Fetches Kubernetes events for the AerospikeCluster CR, scoped via
involvedObject on the server side.

Note: --since is applied client-side after fetching; the REST endpoint has no
'since' query parameter. To widen the window, combine with --limit.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit < 1 || limit > 500 {
				return fmt.Errorf("--limit must be between 1 and 500, got %d", limit)
			}
			ns, name, err := splitNamespacedName(args[0])
			if err != nil {
				return err
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			events, err := c.ListK8sClusterEvents(cmd.Context(), ns, name, limit, category)
			if err != nil {
				return err
			}
			events = filterEventsSince(events, since, time.Now())
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, events,
				output.WithTable(
					[]string{"TYPE", "REASON", "CATEGORY", "MESSAGE", "LAST_SEEN", "COUNT"},
					func(v any) []string {
						e := v.(client.K8sClusterEvent)
						return []string{
							e.Type,
							e.Reason,
							e.Category,
							sanitizeCell(e.Message),
							e.LastTimestamp,
							strconv.Itoa(e.Count),
						}
					},
					func(any) []any {
						rows := make([]any, 0, len(events))
						for _, e := range events {
							rows = append(rows, e)
						}
						return rows
					},
				),
			)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "max events to return (1-500)")
	cmd.Flags().StringVar(&category, "category", "", "filter by event category (e.g. Scaling, Lifecycle, Monitoring, Network, Template, Circuit Breaker, Other)")
	cmd.Flags().DurationVar(&since, "since", 0, "only show events with lastTimestamp newer than this duration (e.g. 15m, 1h). Applied client-side after fetch — REST has no 'since' param")
	return cmd
}

// filterEventsSince keeps events whose effective timestamp is at or after
// now-since. The effective timestamp is lastTimestamp when present and
// parseable, falling back to firstTimestamp for events.k8s.io/v1 EventSeries
// that omit lastTimestamp. Events with neither field parseable are kept so
// the user is not silently shown an incomplete picture when the server
// formats timestamps unexpectedly. Uses `>= cutoff` semantics: an event
// timestamped exactly now-since is included.
func filterEventsSince(events []client.K8sClusterEvent, since time.Duration, now time.Time) []client.K8sClusterEvent {
	if since <= 0 {
		return events
	}
	cutoff := now.Add(-since)
	out := make([]client.K8sClusterEvent, 0, len(events))
	for _, e := range events {
		t, ok := parseEventTimestamp(e)
		if !ok {
			out = append(out, e)
			continue
		}
		if !t.Before(cutoff) {
			out = append(out, e)
		}
	}
	return out
}

// parseEventTimestamp returns the effective timestamp of an event, trying
// lastTimestamp first and falling back to firstTimestamp. The boolean is
// false when neither field is set or parseable.
func parseEventTimestamp(e client.K8sClusterEvent) (time.Time, bool) {
	for _, ts := range []string{e.LastTimestamp, e.FirstTimestamp} {
		if ts == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// newK8sClusterLogsCmd wires `ackoctl k8s cluster logs NAMESPACE/NAME --pod
// POD ...`. Default output is the raw log string (kubectl-style); -o json|yaml
// emits the full response envelope so users can pipe pod/tailLines/sinceSeconds
// into other tools. We validate tail and --since client-side to match the
// FastAPI bounds (1..10000 and 1..86400 seconds) and surface a friendly error
// before round-tripping to the server.
func newK8sClusterLogsCmd(global *GlobalFlags) *cobra.Command {
	var (
		pod       string
		container string
		tail      int
		since     time.Duration
	)
	cmd := &cobra.Command{
		Use:   "logs NAMESPACE/NAME",
		Short: "Fetch logs from a pod in an ACKO-managed cluster",
		Long: `Reads kubelet logs for a single pod owned by the named AerospikeCluster.
By default prints the raw log string to stdout (like 'kubectl logs').
Use -o json or -o yaml to emit the full {pod, logs, tailLines, sinceSeconds}
envelope. Streaming (--follow) is not supported.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ns, name, err := splitNamespacedName(args[0])
			if err != nil {
				return err
			}
			if tail < 1 || tail > 10000 {
				return fmt.Errorf("--tail must be between 1 and 10000, got %d", tail)
			}
			sinceSeconds, err := sinceFlagToSeconds(since)
			if err != nil {
				return err
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			result, err := c.GetK8sPodLogs(cmd.Context(), ns, name, pod, client.K8sLogsOptions{
				Container:    container,
				Tail:         tail,
				SinceSeconds: sinceSeconds,
			})
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			if format == output.FormatTable {
				// Match kubectl: emit only the raw logs payload, preserving any
				// trailing newline the kubelet returned. The envelope (pod,
				// tailLines, sinceSeconds) is JSON/YAML-only metadata.
				_, err := fmt.Fprint(cmd.OutOrStdout(), result.Logs)
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, result)
		},
	}
	cmd.Flags().StringVar(&pod, "pod", "", "pod name (required)")
	cmd.Flags().StringVar(&container, "container", "", "container name within the pod")
	cmd.Flags().IntVar(&tail, "tail", 500, "number of tail lines to return (1..10000)")
	cmd.Flags().DurationVar(&since, "since", 0, "only return logs newer than this duration (e.g. 30m, 1h; max 24h)")
	_ = cmd.MarkFlagRequired("pod")
	return cmd
}

// sinceFlagToSeconds converts a --since duration into the integer seconds the
// FastAPI logs endpoint expects. Zero means "not set" -> 0. Sub-second
// precision is rounded UP so users get at least the requested window;
// truncating down would silently return fewer logs than asked for.
func sinceFlagToSeconds(d time.Duration) (int, error) {
	if d <= 0 {
		return 0, nil
	}
	secs := int(math.Ceil(d.Seconds()))
	if secs < 1 || secs > 86400 {
		return 0, fmt.Errorf("--since must resolve to between 1s and 24h, got %s", d)
	}
	return secs, nil
}

// newK8sClusterPodsCmd wires `ackoctl k8s cluster pods NAMESPACE/NAME`. Default
// table output keeps the columns operators reach for first when triaging a
// cluster (phase/ready/podIP/nodeId/rackId/image); -o json|yaml emits the full
// K8sPodStatus envelope including configHash/podSpecHash/accessEndpoints so
// upgrade-tracking scripts and dashboards can consume the entire shape.
func newK8sClusterPodsCmd(global *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "pods NAMESPACE/NAME",
		Short: "List pod status for an ACKO-managed cluster",
		Long: `Fetches per-pod status for the AerospikeCluster CR via
GET /k8s/clusters/{ns}/{name}/pods. The table view shows the columns
most useful during triage; -o json or -o yaml emits the full payload
(configHash, podSpecHash, accessEndpoints, etc.).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ns, name, err := splitNamespacedName(args[0])
			if err != nil {
				return err
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			pods, err := c.ListK8sPods(cmd.Context(), ns, name)
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, pods,
				output.WithTable(
					[]string{"NAME", "PHASE", "READY", "POD_IP", "NODE_ID", "RACK_ID", "IMAGE"},
					func(v any) []string {
						p := v.(client.K8sPodStatus)
						return []string{
							p.Name,
							p.Phase,
							strconv.FormatBool(p.IsReady),
							p.PodIP,
							p.NodeID,
							rackIDField(p.RackID),
							p.Image,
						}
					},
					func(any) []any {
						rows := make([]any, 0, len(pods))
						for _, p := range pods {
							rows = append(rows, p)
						}
						return rows
					},
				),
			)
		},
	}
}

// rackIDField renders an optional rack id for the table view. Nil prints as
// empty so the column does not flap between "0" (meaning rack 0) and "no rack
// id reported"; CE's rack-aware feature uses 1-indexed rack ids in practice
// but the wire field is technically nullable, so we preserve that distinction.
func rackIDField(rid *int) string {
	if rid == nil {
		return ""
	}
	return strconv.Itoa(*rid)
}

// splitNamespacedName parses a "NAMESPACE/NAME" argument into its two parts.
//
// It splits on the single, required '/' separator. A Kubernetes namespace and
// object name can never themselves contain a '/', so an argument carrying more
// than one slash (e.g. "ns/name/extra" or a trailing "ns/name/") is a user
// mistake and is rejected. strings.Cut alone would silently fold the extra
// segment into name ("ns/name/extra" -> name="name/extra"); that bogus name
// then round-trips as a path-escaped "%2F" segment to cluster-manager, where
// it surfaces as a confusing 404 far from the real cause. Failing fast here
// gives the user an actionable error pointing at the malformed argument.
func splitNamespacedName(s string) (string, string, error) {
	ns, name, ok := strings.Cut(s, "/")
	if !ok || ns == "" || name == "" || strings.Contains(name, "/") {
		return "", "", fmt.Errorf("expected NAMESPACE/NAME, got %q", s)
	}
	return ns, name, nil
}

func stringField(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return missingFieldSentinel
	}
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprintf("%v", t)
	}
}
