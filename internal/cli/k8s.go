package cli

import (
	"fmt"
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
			if cur, ok := intField(current, "size"); ok && size < cur && !yes {
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
// JSON numbers decode as float64; int/int64 are accepted for robustness.
func intField(m map[string]any, key string) (int, bool) {
	v, ok := m[key]
	if !ok || v == nil {
		return 0, false
	}
	switch t := v.(type) {
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
							e.Message,
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

func splitNamespacedName(s string) (string, string, error) {
	ns, name, ok := strings.Cut(s, "/")
	if !ok || ns == "" || name == "" {
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
