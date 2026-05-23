package cli

import (
	"encoding/json"
	"fmt"
	"strings"
)

// validatePKType rejects a --pk-type value that is not one of the particle
// types cluster-manager accepts. An empty string means "not supplied" and is
// allowed — the server defaults it to "auto". Without this check a typo such
// as `--pk-type integer` round-trips to the server and surfaces as an opaque
// 422, often after a non-idempotent write has already been attempted.
func validatePKType(pkType string) error {
	switch pkType {
	case "", "auto", "string", "int", "bytes":
		return nil
	default:
		return fmt.Errorf("--pk-type must be one of auto|string|int|bytes, got %q", pkType)
	}
}

// validateIndexType rejects a secondary-index --type that is not one of the
// types cluster-manager accepts. Unlike --pk-type the value is mandatory, so
// an empty string is also rejected here.
func validateIndexType(idxType string) error {
	switch idxType {
	case "numeric", "string", "geo2dsphere":
		return nil
	default:
		return fmt.Errorf("--type must be one of numeric|string|geo2dsphere, got %q", idxType)
	}
}

// validatePort rejects a TCP port outside the valid 1..65535 range. A port of
// 0 or a value above 65535 is a user typo that cluster-manager would reject
// with a confusing validation error; failing fast client-side is clearer.
func validatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("--port must be between 1 and 65535, got %d", port)
	}
	return nil
}

// validateQueryOp rejects a `query exec --op` value that is not one of the
// predicate operators cluster-manager accepts. An empty string means "no
// predicate" and is handled by the caller, so it is allowed here. Without this
// check a typo such as `--op equal` round-trips to the server and surfaces as
// an opaque 422 only after the query request has been fully assembled.
func validateQueryOp(op string) error {
	switch op {
	case "", "equals", "between", "contains", "geo_within_region", "geo_contains_point":
		return nil
	default:
		return fmt.Errorf("--op must be one of equals|between|contains|geo_within_region|geo_contains_point, got %q", op)
	}
}

// validateJSONObjectFlag rejects a flag value that is not a non-empty JSON
// object literal. json.Unmarshal into map[string]any silently accepts JSON
// `null` (yielding a nil map) and reports "unexpected end of JSON input" for
// empty or whitespace input, both of which are confusing or downright
// dangerous when the resulting nil map is forwarded to the server. flagName
// (e.g. "--bins", "--filter", "--predicate") is interpolated into the error
// messages so callers do not need to wrap the result.
func validateJSONObjectFlag(raw, flagName string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Errorf("%s must be a non-empty JSON object, e.g. '{\"name\":\"Alice\"}'", flagName)
	}
	if trimmed[0] != '{' {
		// Decode into any just to surface the actual top-level JSON kind in
		// the error — e.g. array, string, number, null — so the user can see
		// at a glance why their input was rejected.
		var v any
		if err := json.Unmarshal([]byte(trimmed), &v); err != nil {
			return fmt.Errorf("%s must be a JSON object: %w", flagName, err)
		}
		return fmt.Errorf("%s must be a JSON object, got %s", flagName, jsonKindOf(v))
	}
	return nil
}

// validateBinsJSONObject is a thin wrapper kept for the `record put --bins`
// call site and its existing tests. New call sites should use
// validateJSONObjectFlag directly.
func validateBinsJSONObject(binsJSON string) error {
	return validateJSONObjectFlag(binsJSON, "--bins")
}

// jsonKindOf returns a human-readable name for the top-level JSON value kind
// produced by json.Unmarshal into any. Used only for error messages.
func jsonKindOf(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "bool"
	case float64:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return fmt.Sprintf("%T", v)
	}
}

// validatePKMatchMode rejects a `record query --pk-match-mode` value that is
// not one of the modes cluster-manager accepts. An empty string means "not
// supplied" and is allowed — the server defaults it to "exact". Without this
// check a typo such as `--pk-match-mode prefex` round-trips to the server and
// surfaces as an opaque 422.
func validatePKMatchMode(mode string) error {
	switch mode {
	case "", "exact", "prefix", "regex":
		return nil
	default:
		return fmt.Errorf("--pk-match-mode must be one of exact|prefix|regex, got %q", mode)
	}
}
