package cli

import (
	"cmp"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// decodeJSONNumber decodes a user-supplied JSON flag value into out with
// json.Decoder.UseNumber, so JSON numbers land as json.Number (their exact
// textual form) instead of float64.
//
// This is the request-side twin of the UseNumber decode that
// internal/client/client.go already performs on responses. Aerospike integer
// bins are int64, and a plain json.Unmarshal routes every number through
// float64: any integer outside the 53-bit exactly-representable range
// (snowflake ids, nanosecond timestamps, 64-bit hashes) is silently rounded
// before the request body is marshalled, so the server writes — or queries
// for — the rounded value while ackoctl exits 0. json.Number re-marshals
// verbatim, so the digits the user typed are the digits the server receives,
// and small integers still serialise compactly (`30`, not `30.0`).
//
// Unlike json.Decoder on a stream, trailing data after the first JSON value
// is rejected, matching json.Unmarshal's behaviour so that input such as
// `[1,2] junk` keeps failing instead of being silently truncated.
func decodeJSONNumber(raw string, out any) error {
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(out); err != nil {
		return err
	}
	var trailing json.RawMessage
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("invalid character after top-level JSON value")
	}
	return nil
}

// compareJSONNumbers orders two JSON numbers decoded with UseNumber. It
// compares as int64 when both operands are integral (so bounds beyond
// float64's 53-bit precision compare exactly) and falls back to float64 when
// either side is a float or exponent form such as `1e3`. The bool reports
// whether the pair was numerically comparable at all.
func compareJSONNumbers(a, b json.Number) (int, bool) {
	ai, aErr := a.Int64()
	bi, bErr := b.Int64()
	if aErr == nil && bErr == nil {
		return cmp.Compare(ai, bi), true
	}
	af, aErr := a.Float64()
	bf, bErr := b.Float64()
	if aErr == nil && bErr == nil {
		return cmp.Compare(af, bf), true
	}
	return 0, false
}
