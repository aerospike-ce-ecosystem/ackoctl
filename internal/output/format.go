package output

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
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
		if err := enc.Encode(data); err != nil {
			_ = enc.Close()
			return err
		}
		return enc.Close()
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

	if cfg.row != nil && len(cfg.headers) > 0 {
		fmt.Fprintln(tw, strings.Join(cfg.headers, "\t"))
		items := []any{data}
		if cfg.rows != nil {
			items = cfg.rows(data)
		}
		for _, item := range items {
			fmt.Fprintln(tw, strings.Join(cfg.row(item), "\t"))
		}
		return tw.Flush()
	}

	// Generic fallback: key: value dump.
	if err := writeKeyValue(tw, "", data); err != nil {
		_ = tw.Flush()
		return err
	}
	return tw.Flush()
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
			if err := writeField(w, prefix, name, rv.Field(i).Interface()); err != nil {
				return err
			}
		}
		return nil
	case reflect.Map:
		keys := rv.MapKeys()
		strs := make([]string, len(keys))
		idx := make(map[string]reflect.Value, len(keys))
		for i, k := range keys {
			s := fmt.Sprint(k.Interface())
			strs[i] = s
			idx[s] = k
		}
		sort.Strings(strs)
		for _, s := range strs {
			if err := writeField(w, prefix, s, rv.MapIndex(idx[s]).Interface()); err != nil {
				return err
			}
		}
		return nil
	case reflect.Slice, reflect.Array:
		if rv.Len() == 0 {
			fmt.Fprintf(w, "%s[]\n", prefix)
			return nil
		}
		for i := 0; i < rv.Len(); i++ {
			elem := rv.Index(i).Interface()
			if isComposite(elem) {
				fmt.Fprintf(w, "%s-\n", prefix)
				if err := writeKeyValue(w, prefix+"  ", elem); err != nil {
					return err
				}
				continue
			}
			fmt.Fprintf(w, "%s- %v\n", prefix, elem)
		}
		return nil
	default:
		fmt.Fprintf(w, "%s%v\n", prefix, v)
		return nil
	}
}

func writeField(w io.Writer, prefix, name string, v any) error {
	v = dereference(v)
	if v == nil {
		fmt.Fprintf(w, "%s%s:\n", prefix, name)
		return nil
	}
	if isComposite(v) {
		fmt.Fprintf(w, "%s%s:\n", prefix, name)
		return writeKeyValue(w, prefix+"  ", v)
	}
	fmt.Fprintf(w, "%s%s:\t%v\n", prefix, name, v)
	return nil
}

func isComposite(v any) bool {
	if v == nil {
		return false
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return false
		}
		rv = rv.Elem()
	}
	switch rv.Kind() {
	case reflect.Struct, reflect.Map:
		return true
	case reflect.Slice, reflect.Array:
		return rv.Len() > 0
	default:
		return false
	}
}

// dereference returns the underlying value when v is a non-nil pointer; nil
// for nil pointers; v unchanged otherwise. Without this, table fallback
// renders pointer fields as raw addresses (e.g. 0x79407532b010).
func dereference(v any) any {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return nil
	}
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil
		}
		return rv.Elem().Interface()
	}
	return v
}
