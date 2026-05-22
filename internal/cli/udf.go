package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aerospike-ce-ecosystem/ackoctl/internal/client"
	"github.com/aerospike-ce-ecosystem/ackoctl/internal/output"
)

// maxUDFSourceSize caps how large a Lua source file `udf upload` will read
// into memory before sending it as a JSON string body. 5 MiB is far above
// any realistic Lua module while still bounding the request: without a cap,
// pointing --file at a multi-gigabyte file would balloon ackoctl's memory
// and produce an oversized request the server rejects anyway.
const maxUDFSourceSize = 5 << 20 // 5 MiB

func newUdfCmd(global *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "udf",
		Short: "Manage Aerospike User-Defined Functions (Lua modules)",
		Long: `UDF management routes through cluster-manager's /api/v1/udfs surface,
which proxies aerospike-py's udf_put / udf_remove / udf-list info calls.
Only Lua modules are supported in Aerospike CE.`,
	}
	cmd.AddCommand(
		newUdfListCmd(global),
		newUdfUploadCmd(global),
		newUdfRemoveCmd(global),
	)
	return cmd
}

func newUdfListCmd(global *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list CONN_ID",
		Short: "List registered UDF modules on a cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			modules, err := c.ListUDFs(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, modules,
				output.WithTable(
					[]string{"FILENAME", "TYPE", "HASH"},
					func(v any) []string {
						m := v.(client.UDFModule)
						return []string{m.Filename, m.Type, m.Hash}
					},
					func(any) []any {
						rows := make([]any, 0, len(modules))
						for _, m := range modules {
							rows = append(rows, m)
						}
						return rows
					},
				),
			)
		},
	}
}

func newUdfUploadCmd(global *GlobalFlags) *cobra.Command {
	var (
		filePath string
		filename string
	)
	cmd := &cobra.Command{
		Use:   "upload CONN_ID",
		Short: "Register a Lua UDF module from a local file",
		Long: `Uploads a Lua source file as a registered UDF module on the target cluster.
The request body is JSON {"filename":..., "content":<source>}; this is not a
multipart upload. When --filename is omitted, the basename of --file is used
as the registered module name. cluster-manager validates the filename against
^[a-zA-Z0-9_.-]{1,255}$ — invalid names surface as a 422 error.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Stat first so an oversized or non-regular file is rejected
			// before its bytes are pulled into memory.
			info, err := os.Stat(filePath)
			if err != nil {
				return fmt.Errorf("read udf source: %w", err)
			}
			if info.IsDir() {
				return fmt.Errorf("udf source %s is a directory, not a file", filePath)
			}
			if info.Size() > maxUDFSourceSize {
				return fmt.Errorf("udf source file %s is %d bytes, exceeds the %d byte limit", filePath, info.Size(), maxUDFSourceSize)
			}
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("read udf source: %w", err)
			}
			if len(data) == 0 {
				return fmt.Errorf("udf source file %s is empty", filePath)
			}
			effective := strings.TrimSpace(filename)
			if effective == "" {
				effective = filepath.Base(filePath)
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			module, err := c.UploadUDF(cmd.Context(), args[0], effective, string(data))
			if err != nil {
				return err
			}
			format, err := global.Format()
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), format, module,
				output.WithTable(
					[]string{"FILENAME", "TYPE", "HASH"},
					func(v any) []string {
						// UploadUDF returns *UDFModule; the row callback gets
						// the same value back from Print, so the assertion
						// must match the pointer shape, not the bare struct.
						m := v.(*client.UDFModule)
						return []string{m.Filename, m.Type, m.Hash}
					},
					func(any) []any { return []any{module} },
				),
			)
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "path to a local .lua source file (required)")
	cmd.Flags().StringVar(&filename, "filename", "", "override the registered module name (default: basename of --file)")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func newUdfRemoveCmd(global *GlobalFlags) *cobra.Command {
	var (
		filename string
		yes      bool
	)
	cmd := &cobra.Command{
		Use:   "remove CONN_ID",
		Short: "Remove a registered UDF module by filename",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("confirmation required (--yes)")
			}
			// MarkFlagRequired only checks --filename was supplied, not that it
			// carries a name. Reject empty/whitespace values so the server is
			// never hit with a meaningless removal request.
			filename = strings.TrimSpace(filename)
			if filename == "" {
				return fmt.Errorf("--filename must not be empty")
			}
			c, err := newClient(cmd, global)
			if err != nil {
				return err
			}
			if err := c.RemoveUDF(cmd.Context(), args[0], filename); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Removed UDF module %s from %s\n", filename, args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&filename, "filename", "", "registered module name to remove (required)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "confirm destructive remove")
	_ = cmd.MarkFlagRequired("filename")
	return cmd
}
