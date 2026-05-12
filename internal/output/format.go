package output

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
)

func Parse(s string) (Format, error) {
	switch strings.ToLower(s) {
	case "", "table":
		return FormatTable, nil
	case "json":
		return FormatJSON, nil
	case "yaml", "yml":
		return FormatYAML, nil
	default:
		return "", fmt.Errorf("unknown output format %q (want table|json|yaml)", s)
	}
}

// Print writes data in the requested format. For FormatTable, headers and a
// row extractor must be provided via WithTable; otherwise table output falls
// back to a generic key:value listing for structs and a JSON-like dump for
// other shapes.
func Print(w io.Writer, format Format, data any, opts ...Option) error {
	cfg := options{}
	for _, opt := range opts {
		opt(&cfg)
	}
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	case FormatYAML:
		enc := yaml.NewEncoder(w)
		enc.SetIndent(2)
		defer enc.Close()
		return enc.Encode(data)
	case FormatTable, "":
		return writeTable(w, data, cfg)
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

type options struct {
	headers []string
	row     func(any) []string
	rows    func(any) []any
}

type Option func(*options)

func WithTable(headers []string, rowFn func(any) []string, rowsFn func(any) []any) Option {
	return func(o *options) {
		o.headers = headers
		o.row = rowFn
		o.rows = rowsFn
	}
}

func writeTable(w io.Writer, data any, cfg options) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	defer tw.Flush()

	if cfg.row != nil && len(cfg.headers) > 0 {
		fmt.Fprintln(tw, strings.Join(cfg.headers, "\t"))
		items := []any{data}
		if cfg.rows != nil {
			items = cfg.rows(data)
		}
		for _, item := range items {
			fmt.Fprintln(tw, strings.Join(cfg.row(item), "\t"))
		}
		return nil
	}

	// Generic fallback: key: value dump.
	return writeKeyValue(tw, "", data)
}

func writeKeyValue(w io.Writer, prefix string, v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			fmt.Fprintf(w, "%s<nil>\n", prefix)
			return nil
		}
		rv = rv.Elem()
	}
	switch rv.Kind() {
	case reflect.Struct:
		t := rv.Type()
		for i := 0; i < rv.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			name := f.Tag.Get("yaml")
			if name == "" {
				name = f.Tag.Get("json")
			}
			name, _, _ = strings.Cut(name, ",")
			if name == "-" {
				continue
			}
			if name == "" {
				name = strings.ToLower(f.Name)
			}
			fmt.Fprintf(w, "%s%s:\t%v\n", prefix, name, rv.Field(i).Interface())
		}
		return nil
	case reflect.Slice, reflect.Array:
		for i := 0; i < rv.Len(); i++ {
			fmt.Fprintf(w, "%s- ", prefix)
			if err := writeKeyValue(w, prefix+"  ", rv.Index(i).Interface()); err != nil {
				return err
			}
		}
		return nil
	default:
		fmt.Fprintf(w, "%s%v\n", prefix, v)
		return nil
	}
}
