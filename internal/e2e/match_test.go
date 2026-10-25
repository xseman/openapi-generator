package e2e

import (
	"encoding/json"
	"fmt"
	"maps"
	"math/big"
	"slices"
	"strings"
	"testing"
)

// match compares a scenario's expectation with what arrived, the way the
// scenarios read: an object lists the keys it checks (others may be there),
// an array has exactly its items, "$any" is any value, "$absent" is a key
// that must be missing. In a request, where a query, header or form value is
// a list, a string stands for a list of just that string, and header names
// match in any case. It returns one line per difference.
func match(path string, want, got any) []string {
	switch w := want.(type) {
	case string:
		switch w {
		case "$any":
			return nil
		case "$absent":
			return []string{path + ": want absent, got " + show(got)}
		}

		if g, ok := got.([]any); ok && len(g) == 1 && strings.HasPrefix(path, "request.") {
			got = g[0]
		}

		if g, ok := got.(string); !ok || g != w {
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
				diffs = append(diffs, match(path+"."+k, w[k], gv)...)
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
			diffs = append(diffs, match(fmt.Sprintf("%s[%d]", path, i), w[i], g[i])...)
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
		ok                    bool
	}{
		{"extra keys allowed", "result", `{"a":1}`, `{"a":1,"b":2}`, true},
		{"missing key", "result", `{"a":1}`, `{"b":1}`, false},
		{"key case matters in a result", "result", `{"id":"1"}`, `{"ID":"1"}`, false},
		{"header name in any case", "request.header", `{"X-Request-ID":"r"}`, `{"X-Request-Id":["r"]}`, true},
		{"string for a one-value list in a request", "request.query", `{"q":"a"}`, `{"q":["a"]}`, true},
		{"but not in a result", "result", `"stored"`, `["stored"]`, false},
		{"array exact length", "result", `[1]`, `[1,2]`, false},
		{"$any present", "result", `{"a":"$any"}`, `{"a":null}`, true},
		{"$any missing", "result", `{"a":"$any"}`, `{}`, false},
		{"$absent missing", "result", `{"a":"$absent"}`, `{}`, true},
		{"$absent null is present", "result", `{"a":"$absent"}`, `{"a":null}`, false},
		{"null is null", "result", `null`, `null`, true},
		{"null is not an object", "result", `null`, `{}`, false},
		{"1 and 1.0", "result", `1`, `1.0`, true},
		{"int64 exact", "result", `4611686018427387905`, `4611686018427387904`, false},
		{"past int64 exact", "result", `12345678901234567890`, `12345678901234567891`, false},
		{"number is not a string", "result", `1`, `"1"`, false},
	}

	for _, tt := range tests {
		diffs := match(tt.path, decodeJSON([]byte(tt.want)), decodeJSON([]byte(tt.got)))
		if (len(diffs) == 0) != tt.ok {
			t.Errorf("%s: match(%s, %s) = %v, want ok %t", tt.name, tt.want, tt.got, diffs, tt.ok)
		}
	}
}
