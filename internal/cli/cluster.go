package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/client"
	"github.com/aerospike-ce-ecosystem/ackoctl/internal/output"
)

func newClusterCmd(global *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Inspect and configure Aerospike clusters via cluster-manager",
	}
	cmd.AddCommand(
		newClusterInfoCmd(global),
		newClusterConfigureNamespaceCmd(global),
	)
	return cmd
}

func newClusterInfoCmd(global *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "info CONN_ID",
		Short: "Show cluster nodes, namespaces, and sets",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			info, err := c.ClusterInfo(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			// ClusterInfo is a raw map — table view falls back to key:value
			// dump and may flatten nested fields. Hint at -o json/yaml for
			// callers that need the full payload.
			if global.Verbose && format == output.FormatTable {
				fmt.Fprintln(cmd.ErrOrStderr(), "ackoctl: cluster info is a raw map; use -o json/yaml for the full payload")
			}
			return output.Print(cmd.OutOrStdout(), format, info)
		},
	}
}

func newClusterConfigureNamespaceCmd(global *GlobalFlags) *cobra.Command {
	var (
		nsName string
		params []string
		yes    bool
	)
	cmd := &cobra.Command{
		Use:   "configure-namespace CONN_ID",
		Short: "Tune runtime-mutable params of an existing Aerospike namespace",
		Long: `cluster-manager applies dynamic config changes via asinfo set-config.
Namespaces cannot be created at runtime — they must be defined in aerospike.conf.

cluster-manager reads exactly two knobs from this request: memorySize (bytes)
and replicationFactor. Only the knobs you supply are sent; any other --param
key is rejected, because the server would drop it silently and still answer
200.

The change lands on a running namespace, so this command requires --yes/-y —
whether you supply one knob or both.

Supplying just one of the two carries an extra hazard. A cluster-manager
without the fix in
` + clusterManagerOmittedParamFixURL + `
substitutes its own default for the omitted knob (memorySize=1073741824,
replicationFactor=2) and applies it to the running namespace, so a one-knob
change can resize the namespace as a side effect. Read the current values
first:

  ackoctl info CONN_ID --command 'namespace/<ns>'

For knobs outside those two, use the asinfo passthrough:

  ackoctl info CONN_ID --allow-write \
    --command 'set-config:context=namespace;id=<ns>;high-water-disk-pct=70'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Build first so a mistyped --param is reported as a typo rather
			// than as a missing confirmation. The at-least-one-param guard that
			// used to sit here moved into the builder with the rest of the
			// --param validation.
			req, err := buildConfigureNamespaceRequest(nsName, params)
			if err != nil {
				return err
			}
			// A set-config against a live namespace is as destructive as the
			// eleven pre-existing sites that gate on --yes — a wrong value can
			// push a namespace into eviction or stop-writes. The gate is keyed on
			// impact, so it is unconditional: an explicit memorySize=1000000 is
			// every bit as destructive as an omitted one, and gating only on
			// omission would wave the explicit shrink straight through. --yes is
			// the only affordance; ackoctl has no interactive prompt anywhere, by
			// design, so this can never hang in CI.
			if !yes {
				detail := fmt.Sprintf("configure-namespace mutates a live namespace on %s", mutationTarget(global))
				// When the body is partial, say which knob the server will fill
				// in and with what: that is the difference between a refusal the
				// operator can act on and one they can only work around.
				if omitted := omittedKnobs(req); len(omitted) != 0 {
					detail += fmt.Sprintf(". %s: a cluster-manager without the fix in %s substitutes its own default and applies it to namespace %q, so supply the value explicitly unless you intend that",
						strings.Join(omitted, "; "), clusterManagerOmittedParamFixURL, nsName)
				}
				return fmt.Errorf("confirmation required (--yes): %s", detail)
			}
			// Confirmed, but still partial: name the knob the server may
			// overwrite before the request goes out. Both the gate detail above
			// and this warning become unnecessary once no reachable server
			// predates the linked fix.
			if omitted := omittedKnobs(req); len(omitted) != 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"ackoctl: WARNING — %s. A cluster-manager without the fix in %s will apply its own default to namespace %q on the running cluster\n",
					strings.Join(omitted, "; "), clusterManagerOmittedParamFixURL, nsName)
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			msg, err := c.ConfigureNamespace(cmd.Context(), args[0], req)
			if err != nil {
				return err
			}
			if msg == "" {
				msg = "applied (server returned no message)"
			}
			fmt.Fprintln(cmd.ErrOrStderr(), msg)
			return nil
		},
	}
	cmd.Flags().StringVar(&nsName, "name", "", "namespace name (required)")
	// StringArrayVar (not StringSliceVar) is kept even though both accepted
	// values are plain integers: a value containing a comma must reach the
	// unknown-key guard below and be reported, not be silently split into
	// fragments that produce a different error.
	cmd.Flags().StringArrayVar(&params, "param", nil,
		"config knob as key=value; only memorySize=<bytes> and replicationFactor=<1-8> are read by the server (repeatable)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "confirm live-namespace config mutation")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

// clusterManagerOmittedParamFixURL points at the paired cluster-manager change
// that makes memorySize/replicationFactor optional and emits only the keys the
// caller actually supplied. Until a deployment carries it, an omitted knob is
// replaced by the server's default and applied to the live namespace — which is
// the only reason the partial-body confirmation below exists. Drop the gate, the
// warning and this constant once every reachable server has the fix.
const clusterManagerOmittedParamFixURL = "https://github.com/aerospike-ce-ecosystem/aerospike-cluster-manager/pull/478"

// cluster-manager's CreateNamespaceRequest bounds, mirrored client-side so a
// bad value fails next to the typo instead of after a round-trip. See
// models/cluster.py: memorySize Field(ge=1_000_000), replicationFactor
// Field(ge=1, le=8).
const (
	minNamespaceMemorySize = 1_000_000
	minReplicationFactor   = 1
	maxReplicationFactor   = 8
)

// Server-side defaults, quoted back to the operator when a knob is omitted.
// These are what a pre-fix cluster-manager substitutes before interpolating the
// value into a live set-config.
const (
	serverDefaultMemorySize        = 1_073_741_824
	serverDefaultReplicationFactor = 2
)

// omittedKnobs describes the accepted knobs the operator did not supply, in a
// stable order, each paired with the default a pre-fix cluster-manager puts
// there instead. Empty when the body is complete, so callers can treat a
// non-empty result as "this request depends on server-side defaults".
func omittedKnobs(req client.ConfigureNamespaceRequest) []string {
	var out []string
	if req.MemorySize == nil {
		out = append(out, fmt.Sprintf("memorySize was not supplied (server default: %d bytes)", serverDefaultMemorySize))
	}
	if req.ReplicationFactor == nil {
		out = append(out, fmt.Sprintf("replicationFactor was not supplied (server default: %d)", serverDefaultReplicationFactor))
	}
	return out
}

// buildConfigureNamespaceRequest turns --param key=value pairs into the typed
// request body, carrying only the knobs the operator supplied.
//
// The allowlist is a data-availability guard, not tidiness. cluster-manager's
// CreateNamespaceRequest declares only memorySize/replicationFactor plus `name`
// and sets no `extra=`, so Pydantic's default extra="ignore" drops every other
// key — then the handler interpolates body.memorySize and
// body.replicationFactor, defaults included, into
//
//	set-config:context=namespace;id=<ns>;memory-size=…;replication-factor=…
//
// against the running cluster and returns 200. Before this guard, ackoctl's own
// documented example (--param=high-water-disk-pct=70 --param=stop-writes-pct=90)
// dropped both knobs AND shrank the namespace to 1 GiB at replication factor 2,
// while the CLI printed "configured successfully" and exited 0.
//
// A partial body is deliberately allowed. Requiring both knobs would force an
// operator changing only the replication factor to restate a memory size they
// may have to guess, which is the same hazard by another route. The risk that an
// omitted knob is filled in by a pre-fix server is handled by the caller's
// confirmation gate, not by refusing here.
func buildConfigureNamespaceRequest(nsName string, params []string) (client.ConfigureNamespaceRequest, error) {
	// The cluster-manager body schema names the namespace field `name` (see
	// CreateNamespaceRequest in api/models/cluster.py). An earlier build sent
	// `namespace`, which the server rejected with HTTP 422
	// ({"loc":["body","name"],"msg":"Field required"}).
	req := client.ConfigureNamespaceRequest{Name: nsName}

	// Require at least one --param: a request carrying only the namespace name
	// is a no-op the server would either reject or silently apply nothing for.
	// Failing fast tells the user the command did nothing because they forgot
	// to pass a parameter.
	if len(params) == 0 {
		return req, fmt.Errorf("at least one --param key=value is required")
	}

	for _, p := range params {
		k, v, ok := strings.Cut(p, "=")
		if !ok {
			return req, fmt.Errorf("invalid --param %q (expected key=value)", p)
		}
		// strings.Cut("=90", "=") yields an empty key with ok=true; reject it
		// explicitly rather than letting it fall through to the unknown-key
		// error, where a quoted "" reads as a puzzle.
		if k == "" {
			return req, fmt.Errorf("invalid --param %q: key must not be empty", p)
		}
		if k == "name" {
			return req, fmt.Errorf("--param name=... is reserved; use --name to set the namespace")
		}

		switch k {
		case "memorySize":
			// A repeated key would silently overwrite the earlier value
			// (--param x=1 --param x=2 quietly drops x=1). For a config
			// mutation that is a foot-gun, so reject the collision.
			if req.MemorySize != nil {
				return req, fmt.Errorf("--param %q specified more than once", k)
			}
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return req, fmt.Errorf("--param memorySize=%q must be a byte count in decimal digits (e.g. 2147483648 for 2 GiB)", v)
			}
			if n < minNamespaceMemorySize {
				return req, fmt.Errorf("--param memorySize=%d is below the server minimum of %d bytes", n, minNamespaceMemorySize)
			}
			req.MemorySize = &n
		case "replicationFactor":
			if req.ReplicationFactor != nil {
				return req, fmt.Errorf("--param %q specified more than once", k)
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return req, fmt.Errorf("--param replicationFactor=%q must be an integer between %d and %d", v, minReplicationFactor, maxReplicationFactor)
			}
			if n < minReplicationFactor || n > maxReplicationFactor {
				return req, fmt.Errorf("--param replicationFactor=%d is outside the server range %d-%d", n, minReplicationFactor, maxReplicationFactor)
			}
			req.ReplicationFactor = &n
		default:
			return req, fmt.Errorf(
				"--param %q would be silently ignored: cluster-manager reads only memorySize and replicationFactor from this request. "+
					"Apply other knobs with `ackoctl info CONN_ID --allow-write --command 'set-config:context=namespace;id=%s;%s=%s'`",
				k, nsName, k, v)
		}
	}
	return req, nil
}
