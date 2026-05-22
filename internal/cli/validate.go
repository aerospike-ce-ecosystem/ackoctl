package cli

import "fmt"

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
