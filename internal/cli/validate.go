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
