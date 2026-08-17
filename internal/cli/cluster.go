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
	)
	cmd := &cobra.Command{
		Use:   "configure-namespace CONN_ID",
		Short: "Tune runtime-mutable params of an existing Aerospike namespace",
		Long: `cluster-manager applies dynamic config changes via asinfo set-config.
Namespaces cannot be created at runtime — they must be defined in aerospike.conf.

cluster-manager reads exactly two knobs from this request: memorySize (bytes)
and replicationFactor. Both must be supplied — the server substitutes its own
defaults (memorySize=1073741824, replicationFactor=2) for anything omitted and
applies them to the running namespace, so a partial request would resize it.
Any other --param key is rejected: the server would drop it silently.

For knobs outside those two, use the asinfo passthrough:

  ackoctl info CONN_ID --allow-write \
    --command 'set-config:context=namespace;id=<ns>;high-water-disk-pct=70'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req, err := buildConfigureNamespaceRequest(nsName, params)
			if err != nil {
				return err
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
	// unknown-key/parse guards below and be reported, not be silently split
	// into fragments that produce a different error.
	cmd.Flags().StringArrayVar(&params, "param", nil,
		"config knob as key=value; only memorySize=<bytes> and replicationFactor=<1-8> are read by the server, and both are required")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

// cluster-manager's CreateNamespaceRequest bounds, mirrored client-side so a
// bad value fails next to the typo instead of after a round-trip. See
// models/cluster.py: memorySize Field(ge=1_000_000), replicationFactor
// Field(ge=1, le=8).
const (
	minNamespaceMemorySize = 1_000_000
	minReplicationFactor   = 1
	maxReplicationFactor   = 8
)

// Server-side defaults for the two knobs, quoted in the error that demands both
// be supplied. These are what an omitted field is replaced with before the
// handler interpolates it into a live set-config.
const (
	serverDefaultMemorySize        = 1_073_741_824
	serverDefaultReplicationFactor = 2
)

// buildConfigureNamespaceRequest turns --param key=value pairs into the typed
// request body.
//
// The restriction to memorySize/replicationFactor is a data-availability guard,
// not tidiness. cluster-manager's CreateNamespaceRequest declares only those
// two knobs plus `name` and sets no `extra=`, so Pydantic's default
// extra="ignore" drops every other key — then the handler interpolates
// body.memorySize and body.replicationFactor, defaults included, into
//
//	set-config:context=namespace;id=<ns>;memory-size=…;replication-factor=…
//
// against the running cluster and returns 200. Before this guard, ackoctl's own
// documented example (--param=high-water-disk-pct=70 --param=stop-writes-pct=90)
// dropped both knobs AND shrank the namespace to 1 GiB at replication factor 2,
// while the CLI printed "configured successfully" and exited 0.
//
// Both fields are therefore required rather than optional-and-omitted: with an
// unpatched server, omitting one is indistinguishable from asking for the
// default, and the default is applied to a live namespace. Requiring both means
// every value the server acts on is one the operator typed.
func buildConfigureNamespaceRequest(nsName string, params []string) (client.ConfigureNamespaceRequest, error) {
	// The cluster-manager body schema names the namespace field `name` (see
	// CreateNamespaceRequest in api/models/cluster.py). An earlier build sent
	// `namespace`, which the server rejected with HTTP 422
	// ({"loc":["body","name"],"msg":"Field required"}).
	req := client.ConfigureNamespaceRequest{Name: nsName}
	var memorySet, rfSet bool

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
			if memorySet {
				return req, fmt.Errorf("--param %q specified more than once", k)
			}
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return req, fmt.Errorf("--param memorySize=%q must be a byte count in decimal digits (e.g. 2147483648 for 2 GiB)", v)
			}
			if n < minNamespaceMemorySize {
				return req, fmt.Errorf("--param memorySize=%d is below the server minimum of %d bytes", n, minNamespaceMemorySize)
			}
			req.MemorySize, memorySet = n, true
		case "replicationFactor":
			if rfSet {
				return req, fmt.Errorf("--param %q specified more than once", k)
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return req, fmt.Errorf("--param replicationFactor=%q must be an integer between %d and %d", v, minReplicationFactor, maxReplicationFactor)
			}
			if n < minReplicationFactor || n > maxReplicationFactor {
				return req, fmt.Errorf("--param replicationFactor=%d is outside the server range %d-%d", n, minReplicationFactor, maxReplicationFactor)
			}
			req.ReplicationFactor, rfSet = n, true
		default:
			return req, fmt.Errorf(
				"--param %q would be silently ignored: cluster-manager reads only memorySize and replicationFactor from this request. "+
					"Apply other knobs with `ackoctl info CONN_ID --allow-write --command 'set-config:context=namespace;id=%s;%s=%s'`",
				k, nsName, k, v)
		}
	}

	if !memorySet || !rfSet {
		return req, fmt.Errorf(
			"--param memorySize=<bytes> and --param replicationFactor=<%d-%d> are both required: "+
				"cluster-manager replaces an omitted field with its own default (memorySize=%d, replicationFactor=%d) "+
				"and applies it to the running namespace, so a partial request would resize it",
			minReplicationFactor, maxReplicationFactor, serverDefaultMemorySize, serverDefaultReplicationFactor)
	}
	return req, nil
}
