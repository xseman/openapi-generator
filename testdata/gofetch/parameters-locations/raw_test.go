package parameterslocations

import (
	"encoding/json"
	"net/url"
	"testing"
)

// TestRawMessageParam checks that raw JSON, the type of an inline union,
// goes out as one value: a JSON string unquoted, anything else as written.
func TestRawMessageParam(t *testing.T) {
	q := url.Values{}
	if err := addQuery(q, "s", json.RawMessage(`"abc"`), "form", true); err != nil {
		t.Fatal(err)
	}

	if err := addQuery(q, "n", json.RawMessage(`5`), "form", true); err != nil {
		t.Fatal(err)
	}

	if got := q.Encode(); got != "n=5&s=abc" {
		t.Errorf("query = %s, want n=5&s=abc", got)
	}
}
