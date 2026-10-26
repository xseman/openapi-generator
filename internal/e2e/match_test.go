package e2e

import (
	"encoding/json"
	"fmt"
	"maps"
	"math/big"
	"mime"
	"slices"
	"strings"
	"testing"
	"time"
)

// rules are how strictly match compares. Exact, for the generated cases,
// allows no other keys and no string for a list. Millis is for a client
// whose date-times are JavaScript Dates: they keep the instant to the
// millisecond, not the offset or the digits past it.
type rules struct{ exact, millis bool }

// match compares a scenario's expectation with what arrived, the way the
// scenarios read: an object lists the keys it checks (others may be there),
// an array has exactly its items, "$any" is any value, "$absent" is a key
// that must be missing. In a request, where a query, header or form value is
// a list, a string stands for a list of just that string, and header names
// match in any case. It returns one line per difference.
func match(path string, want, got any, r rules) []string {
	switch w := want.(type) {
	case string:
		switch w {
		case "$any":
			return nil
		case "$absent":
			return []string{path + ": want absent, got " + show(got)}
		}

		if g, ok := got.([]any); ok && len(g) == 1 && !r.exact && strings.HasPrefix(path, "request.") {
			got = g[0]
		}

		// A Content-Type's parameters (a charset fetch adds) do not count.
		if g, ok := got.(string); ok && strings.EqualFold(path, "request.header.Content-Type") {
			got, _, _ = mime.ParseMediaType(g)
		}

		if g, ok := got.(string); !ok || g != w && (!r.millis || !sameMillis(w, g)) {
			return []string{path + ": want " + show(w) + ", got " + show(got)}
		}

		return nil

	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return []string{path + ": want an object, got " + show(got)}
		}

		var diffs []string

		for _, k := range slices.Sorted(maps.Keys(w)) {
			gv, present := g[k]
			if !present && path == "request.header" {
				gv, present = headerValue(g, k)
			}

			switch {
			case w[k] == "$absent":
				if present {
					diffs = append(diffs, path+"."+k+": want absent, got "+show(gv))
				}

			case !present:
				diffs = append(diffs, path+"."+k+": missing, want "+show(w[k]))
			default:
				diffs = append(diffs, match(path+"."+k, w[k], gv, r)...)
			}
		}

		for _, k := range slices.Sorted(maps.Keys(g)) {
			if _, wanted := w[k]; r.exact && !wanted {
				diffs = append(diffs, path+"."+k+": want absent, got "+show(g[k]))
			}
		}

		return diffs

	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return []string{path + ": want " + show(w) + ", got " + show(got)}
		}

		var diffs []string
		for i := range w {
			diffs = append(diffs, match(fmt.Sprintf("%s[%d]", path, i), w[i], g[i], r)...)
		}

		return diffs

	case json.Number:
		if g, ok := got.(json.Number); !ok || !sameNumber(w, g) {
			return []string{path + ": want " + w.String() + ", got " + show(got)}
		}

		return nil
	}

	if want != got {
		return []string{path + ": want " + show(want) + ", got " + show(got)}
	}

	return nil
}

// headerValue finds header k in any case: names on the wire are canonical
// (X-Request-Id) where a scenario may write the spec's (X-Request-ID).
func headerValue(m map[string]any, k string) (any, bool) {
	for mk, v := range m {
		if strings.EqualFold(mk, k) {
			return v, true
		}
	}

	return nil, false
}

// sameNumber compares two JSON numbers exactly: 1 and 1.0 are the same,
// 2^62+1 and 2^62 are not, nor are two integers past int64 that differ.
func sameNumber(a, b json.Number) bool {
	x, okA := new(big.Rat).SetString(a.String())
	y, okB := new(big.Rat).SetString(b.String())

	return okA && okB && x.Cmp(y) == 0
}

// sameMillis says whether two RFC 3339 date-times are one instant to the
// millisecond: 03:04:05Z and 03:04:05.000Z, 23:59:59.123456+05:30 and
// 18:29:59.123Z.
func sameMillis(a, b string) bool {
	x, errA := time.Parse(time.RFC3339Nano, a)
	y, errB := time.Parse(time.RFC3339Nano, b)

	return errA == nil && errB == nil && x.Truncate(time.Millisecond).Equal(y.Truncate(time.Millisecond))
}

func show(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}

	return string(b)
}

// TestMatch pins the matcher's rules; it runs without E2E.
func TestMatch(t *testing.T) {
	tests := []struct {
		name, path, want, got string
		exact, ok             bool
	}{
		{"extra keys allowed", "result", `{"a":1}`, `{"a":1,"b":2}`, false, true},
		{"missing key", "result", `{"a":1}`, `{"b":1}`, false, false},
		{"key case matters in a result", "result", `{"id":"1"}`, `{"ID":"1"}`, false, false},
		{"header name in any case", "request.header", `{"X-Request-ID":"r"}`, `{"X-Request-Id":["r"]}`, false, true},
		{"a Content-Type by its media type", "request.header", `{"Content-Type":"a/b"}`, `{"Content-Type":["a/b;charset=UTF-8"]}`, false, true},
		{"string for a one-value list in a request", "request.query", `{"q":"a"}`, `{"q":["a"]}`, false, true},
		{"but not in a result", "result", `"stored"`, `["stored"]`, false, false},
		{"array exact length", "result", `[1]`, `[1,2]`, false, false},
		{"$any present", "result", `{"a":"$any"}`, `{"a":null}`, false, true},
		{"$any missing", "result", `{"a":"$any"}`, `{}`, false, false},
		{"$absent missing", "result", `{"a":"$absent"}`, `{}`, false, true},
		{"$absent null is present", "result", `{"a":"$absent"}`, `{"a":null}`, false, false},
		{"null is null", "result", `null`, `null`, false, true},
		{"null is not an object", "result", `null`, `{}`, false, false},
		{"1 and 1.0", "result", `1`, `1.0`, false, true},
		{"int64 exact", "result", `4611686018427387905`, `4611686018427387904`, false, false},
		{"past int64 exact", "result", `12345678901234567890`, `12345678901234567891`, false, false},
		{"number is not a string", "result", `1`, `"1"`, false, false},
		{"exact: same keys", "result", `{"a":{"b":null}}`, `{"a":{"b":null}}`, true, true},
		{"exact: no extra key", "result", `{"a":1}`, `{"a":1,"b":2}`, true, false},
		{"exact: no extra key nested", "result", `[{"a":1}]`, `[{"a":1,"b":null}]`, true, false},
		{"exact: null is not absent", "result", `{"a":null}`, `{}`, true, false},
		{"exact: absent is not null", "result", `{}`, `{"a":null}`, true, false},
		{"exact: no string for a list", "request.body", `{"q":"a"}`, `{"q":["a"]}`, true, false},
		{"exact: $any still any", "error", `{"status":500,"body":"$any"}`, `{"status":500,"body":"x"}`, true, true},
		{"date-time digits matter", "result", `"2024-01-02T03:04:05Z"`, `"2024-01-02T03:04:05.000Z"`, true, false},
	}

	for _, tt := range tests {
		diffs := match(tt.path, decodeJSON([]byte(tt.want)), decodeJSON([]byte(tt.got)), rules{exact: tt.exact})
		if (len(diffs) == 0) != tt.ok {
			t.Errorf("%s: match(%s, %s, exact %t) = %v, want ok %t", tt.name, tt.want, tt.got, tt.exact, diffs, tt.ok)
		}
	}

	millis := rules{exact: true, millis: true}
	for _, tt := range []struct {
		want, got string
		ok        bool
	}{
		{`"2024-02-29T23:59:59.123456+05:30"`, `"2024-02-29T18:29:59.123Z"`, true},
		{`"2024-01-02T03:04:05Z"`, `"2024-01-02T03:04:06.000Z"`, false},
		{`"2024-01-02T03:04:05"`, `"2024-01-02T02:04:05.000Z"`, false}, // naive: text
	} {
		if diffs := match("result", decodeJSON([]byte(tt.want)), decodeJSON([]byte(tt.got)), millis); (len(diffs) == 0) != tt.ok {
			t.Errorf("millis: match(%s, %s) = %v, want ok %t", tt.want, tt.got, diffs, tt.ok)
		}
	}
}
