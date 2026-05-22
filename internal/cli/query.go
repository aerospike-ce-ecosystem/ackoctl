package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/client"
	"github.com/aerospike-ce-ecosystem/ackoctl/internal/output"
)

func newQueryCmd(global *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Execute Aerospike queries via cluster-manager",
	}
	cmd.AddCommand(newQueryExecCmd(global))
	return cmd
}

func newQueryExecCmd(global *GlobalFlags) *cobra.Command {
	var (
		namespace, set, bin, op, valueRaw, value2Raw string
		expression, primaryKey, pkType               string
		selectBins                                   []string
		maxRecords                                   int
	)
	cmd := &cobra.Command{
		Use:   "exec CONN_ID",
		Short: "Execute a query (predicate, primary-key lookup, or full scan)",
		Long: `Use --bin/--op/--value (and --value2 for between) to build a predicate,
or --primary-key for a direct lookup. With neither, the query performs a full
scan limited by --max-records. --value/--value2 are parsed as JSON so the
correct particle type (number, string, list, etc.) reaches the server.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate --max-records client-side: 0 means "server default",
			// otherwise it must fall within the documented 1..1000000 range.
			if maxRecords < 0 || maxRecords > 1000000 {
				return fmt.Errorf("--max-records must be 0 (server default) or between 1 and 1000000, got %d", maxRecords)
			}
			if err := validatePKType(pkType); err != nil {
				return err
			}
			req := client.QueryRequest{
				Namespace:  namespace,
				Set:        set,
				Expression: expression,
				SelectBins: selectBins,
				MaxRecords: maxRecords,
				PrimaryKey: primaryKey,
				PKType:     pkType,
			}
			if op != "" || bin != "" || valueRaw != "" || value2Raw != "" {
				if bin == "" || op == "" {
					return fmt.Errorf("--bin and --op are required together when building a predicate")
				}
				// Every supported operator (equals, between, contains, geo_*)
				// needs at least one operand, so --value is always required.
				if valueRaw == "" {
					return fmt.Errorf("--value is required when building a predicate")
				}
				if op == "between" {
					if value2Raw == "" {
						return fmt.Errorf("--value2 is required when --op is 'between'")
					}
				} else if value2Raw != "" {
					return fmt.Errorf("--value2 is only valid when --op is 'between'")
				}
				pred := &client.QueryPredicate{Bin: bin, Operator: op}
				v, err := parseJSONScalar(valueRaw)
				if err != nil {
					return fmt.Errorf("--value: %w", err)
				}
				pred.Value = v
				if value2Raw != "" {
					v2, err := parseJSONScalar(value2Raw)
					if err != nil {
						return fmt.Errorf("--value2: %w", err)
					}
					pred.Value2 = v2
				}
				req.Predicate = pred
			}

			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			resp, err := c.ExecuteQuery(cmd.Context(), args[0], req)
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, resp)
		},
	}
	cmd.Flags().StringVar(&namespace, "namespace", "", "namespace (required)")
	cmd.Flags().StringVar(&set, "set", "", "set scope")
	cmd.Flags().StringVar(&bin, "bin", "", "bin name for predicate")
	cmd.Flags().StringVar(&op, "op", "", "operator: equals|between|contains|geo_within_region|geo_contains_point")
	cmd.Flags().StringVar(&valueRaw, "value", "", "predicate value as JSON (e.g. 30, \"alice\", [1,2])")
	cmd.Flags().StringVar(&value2Raw, "value2", "", "second value for between operator (JSON)")
	cmd.Flags().StringVar(&expression, "expression", "", "raw Aerospike expression (server-side filter)")
	cmd.Flags().StringSliceVar(&selectBins, "select", nil, "bin names to project (comma-separated, repeatable)")
	cmd.Flags().IntVar(&maxRecords, "max-records", 0, "max records to scan (1-1000000)")
	cmd.Flags().StringVar(&primaryKey, "primary-key", "", "primary key for direct lookup")
	cmd.Flags().StringVar(&pkType, "pk-type", "", "particle type for primary key: auto|string|int|bytes")
	_ = cmd.MarkFlagRequired("namespace")
	return cmd
}

// parseJSONScalar accepts either a JSON literal (`"foo"`, `30`, `true`,
// `[1,2]`, `{"k":1}`) or an unquoted bareword that we treat as a plain
// string for UX (`--value alice`).
//
// When the input *looks* like JSON (starts with a JSON literal opener) but
// fails to parse, we surface the error instead of silently downgrading to a
// string, so a typo like `--value '[1,2'` does not end up as the literal
// string predicate `"[1,2"` on the server.
func parseJSONScalar(s string) (any, error) {
	trimmed := strings.TrimSpace(s)
	if looksLikeJSON(trimmed) {
		var v any
		if err := json.Unmarshal([]byte(trimmed), &v); err != nil {
			return nil, fmt.Errorf("looks like JSON but did not parse: %w", err)
		}
		return v, nil
	}
	var v any
	if err := json.Unmarshal([]byte(trimmed), &v); err == nil {
		return v, nil
	}
	return s, nil
}

func looksLikeJSON(s string) bool {
	if s == "" {
		return false
	}
	switch s[0] {
	case '{', '[', '"':
		return true
	}
	// JSON literals: true/false/null.
	switch s {
	case "true", "false", "null":
		return true
	}
	// Numeric: leading digit, minus, or plus sign followed by a digit or dot.
	c := s[0]
	if c == '-' || c == '+' {
		if len(s) > 1 && (isDigit(s[1]) || s[1] == '.') {
			return true
		}
		return false
	}
	return isDigit(c)
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
