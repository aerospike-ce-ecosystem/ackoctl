package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/client"
	"github.com/aerospike-ce-ecosystem/ackoctl/internal/output"
)

func newRecordCmd(global *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "record",
		Short: "Read and write Aerospike records via cluster-manager",
	}
	cmd.AddCommand(
		newRecordListCmd(global),
		newRecordGetCmd(global),
		newRecordPutCmd(global),
		newRecordDeleteCmd(global),
		newRecordDeleteBinCmd(global),
		newRecordQueryCmd(global),
	)
	return cmd
}

func newRecordListCmd(global *GlobalFlags) *cobra.Command {
	var (
		namespace string
		set       string
		pageSize  int
	)
	cmd := &cobra.Command{
		Use:   "list CONN_ID",
		Short: "List records in a namespace/set",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			page, err := c.ListRecords(cmd.Context(), args[0], namespace, set, pageSize)
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, page,
				output.WithTable(
					[]string{"NAMESPACE", "SET", "PK", "GEN", "TTL", "BINS"},
					func(v any) []string {
						r := v.(client.AerospikeRecord)
						return []string{r.Key.Namespace, r.Key.Set, r.Key.PK, strconv.Itoa(r.Meta.Generation), strconv.Itoa(r.Meta.TTL), strconv.Itoa(len(r.Bins))}
					},
					func(any) []any {
						rows := make([]any, 0, len(page.Records))
						for _, r := range page.Records {
							rows = append(rows, r)
						}
						return rows
					},
				),
			)
		},
	}
	cmd.Flags().StringVar(&namespace, "namespace", "", "namespace (required)")
	cmd.Flags().StringVar(&set, "set", "", "set name; empty matches the namespace-default set")
	cmd.Flags().IntVar(&pageSize, "page-size", 25, "max records to return (1-500)")
	_ = cmd.MarkFlagRequired("namespace")
	return cmd
}

func newRecordGetCmd(global *GlobalFlags) *cobra.Command {
	var (
		namespace, set, pk, pkType string
	)
	cmd := &cobra.Command{
		Use:   "get CONN_ID",
		Short: "Get a single record by primary key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			rec, err := c.GetRecord(cmd.Context(), args[0], namespace, set, pk, pkType)
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, rec)
		},
	}
	cmd.Flags().StringVar(&namespace, "namespace", "", "namespace (required)")
	cmd.Flags().StringVar(&set, "set", "", "set (required)")
	cmd.Flags().StringVar(&pk, "pk", "", "primary key (required)")
	cmd.Flags().StringVar(&pkType, "pk-type", "", "particle type for pk: auto|string|int|bytes (default auto)")
	_ = cmd.MarkFlagRequired("namespace")
	_ = cmd.MarkFlagRequired("set")
	_ = cmd.MarkFlagRequired("pk")
	return cmd
}

func newRecordPutCmd(global *GlobalFlags) *cobra.Command {
	var (
		namespace, set, pk, pkType, binsJSON string
		ttl                                  int
	)
	cmd := &cobra.Command{
		Use:   "put CONN_ID",
		Short: "Create or replace a record",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bins := map[string]any{}
			if err := json.Unmarshal([]byte(binsJSON), &bins); err != nil {
				return fmt.Errorf("--bins must be a JSON object: %w", err)
			}
			req := client.RecordWriteRequest{
				Key:    client.RecordKey{Namespace: namespace, Set: set, PK: pk},
				Bins:   bins,
				PKType: pkType,
			}
			if cmd.Flags().Changed("ttl") {
				v := ttl
				req.TTL = &v
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			rec, err := c.PutRecord(cmd.Context(), args[0], req)
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, rec)
		},
	}
	cmd.Flags().StringVar(&namespace, "namespace", "", "namespace (required)")
	cmd.Flags().StringVar(&set, "set", "", "set (required)")
	cmd.Flags().StringVar(&pk, "pk", "", "primary key (required)")
	cmd.Flags().StringVar(&binsJSON, "bins", "", "bins as a JSON object, e.g. '{\"name\":\"Alice\",\"age\":30}' (required)")
	cmd.Flags().StringVar(&pkType, "pk-type", "", "particle type for pk: auto|string|int|bytes (default auto)")
	cmd.Flags().IntVar(&ttl, "ttl", 0, "record TTL in seconds (0 = use namespace default, omitted = server default)")
	_ = cmd.MarkFlagRequired("namespace")
	_ = cmd.MarkFlagRequired("set")
	_ = cmd.MarkFlagRequired("pk")
	_ = cmd.MarkFlagRequired("bins")
	return cmd
}

func newRecordDeleteCmd(global *GlobalFlags) *cobra.Command {
	var (
		namespace, set, pk, pkType string
		yes                        bool
	)
	cmd := &cobra.Command{
		Use:   "delete CONN_ID",
		Short: "Delete a record (idempotent)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("confirmation required (--yes)")
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			if err := c.DeleteRecord(cmd.Context(), args[0], namespace, set, pk, pkType); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Deleted %s.%s pk=%s\n", namespace, set, pk)
			return nil
		},
	}
	cmd.Flags().StringVar(&namespace, "namespace", "", "namespace (required)")
	cmd.Flags().StringVar(&set, "set", "", "set (required)")
	cmd.Flags().StringVar(&pk, "pk", "", "primary key (required)")
	cmd.Flags().StringVar(&pkType, "pk-type", "", "particle type for pk: auto|string|int|bytes (default auto)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "confirm destructive delete")
	_ = cmd.MarkFlagRequired("namespace")
	_ = cmd.MarkFlagRequired("set")
	_ = cmd.MarkFlagRequired("pk")
	return cmd
}

func newRecordDeleteBinCmd(global *GlobalFlags) *cobra.Command {
	var (
		namespace, set, pk, binName, pkType string
		yes                                 bool
	)
	cmd := &cobra.Command{
		Use:   "delete-bin CONN_ID",
		Short: "Delete a single bin from a record (removes whole record if last bin)",
		Long: `Remove one bin from a record. The cluster-manager endpoint is idempotent
on the bin name. Note: removing the last bin from a record causes the entire
record to disappear server-side — this is standard Aerospike behaviour.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("confirmation required (--yes)")
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			if err := c.DeleteBin(cmd.Context(), args[0], namespace, set, pk, binName, pkType); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Deleted bin %q from %s.%s pk=%s\n", binName, namespace, set, pk)
			return nil
		},
	}
	cmd.Flags().StringVar(&namespace, "namespace", "", "namespace (required)")
	cmd.Flags().StringVar(&set, "set", "", "set (required)")
	cmd.Flags().StringVar(&pk, "pk", "", "primary key (required)")
	cmd.Flags().StringVar(&binName, "bin", "", "bin name to delete (required)")
	cmd.Flags().StringVar(&pkType, "pk-type", "", "particle type for pk: auto|string|int|bytes (default auto)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "confirm destructive delete")
	_ = cmd.MarkFlagRequired("namespace")
	_ = cmd.MarkFlagRequired("set")
	_ = cmd.MarkFlagRequired("pk")
	_ = cmd.MarkFlagRequired("bin")
	return cmd
}

func newRecordQueryCmd(global *GlobalFlags) *cobra.Command {
	var (
		namespace, set, pkPattern, pkMatchMode, pkType, filterJSON, predicateJSON string
		page, pageSize, maxRecords                                                int
		selectBins                                                                []string
	)
	cmd := &cobra.Command{
		Use:   "query CONN_ID",
		Short: "Filtered record scan with optional pk pattern and bin filters",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := client.FilteredQueryRequest{
				Namespace:   namespace,
				Set:         set,
				PKPattern:   pkPattern,
				PKMatchMode: pkMatchMode,
				PKType:      pkType,
				Page:        page,
				PageSize:    pageSize,
				MaxRecords:  maxRecords,
				SelectBins:  selectBins,
			}
			if filterJSON != "" {
				m := map[string]any{}
				if err := json.Unmarshal([]byte(filterJSON), &m); err != nil {
					return fmt.Errorf("--filter must be a JSON object: %w", err)
				}
				req.Filters = m
			}
			if predicateJSON != "" {
				m := map[string]any{}
				if err := json.Unmarshal([]byte(predicateJSON), &m); err != nil {
					return fmt.Errorf("--predicate must be a JSON object: %w", err)
				}
				req.Predicate = m
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			resp, err := c.FilterRecords(cmd.Context(), args[0], req)
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
	cmd.Flags().StringVar(&set, "set", "", "set scope (required for pk lookups)")
	cmd.Flags().StringVar(&pkPattern, "pk-pattern", "", "primary key pattern (exact|prefix|regex per --pk-match-mode)")
	cmd.Flags().StringVar(&pkMatchMode, "pk-match-mode", "", "exact|prefix|regex (default exact)")
	cmd.Flags().StringVar(&pkType, "pk-type", "", "particle type for pk: auto|string|int|bytes")
	cmd.Flags().IntVar(&page, "page", 1, "page number (1-based)")
	cmd.Flags().IntVar(&pageSize, "page-size", 25, "page size (1-500)")
	cmd.Flags().IntVar(&maxRecords, "max-records", 0, "global cap across pages (0 = server default)")
	cmd.Flags().StringSliceVar(&selectBins, "select", nil, "comma-separated bin names to project")
	cmd.Flags().StringVar(&filterJSON, "filter", "", "filter group as JSON (cluster-manager FilterGroup shape)")
	cmd.Flags().StringVar(&predicateJSON, "predicate", "", "predicate as JSON (cluster-manager QueryPredicate shape)")
	_ = cmd.MarkFlagRequired("namespace")
	return cmd
}
