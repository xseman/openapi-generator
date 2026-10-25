package nullablerequired

import (
	"encoding/json"
	"testing"
)

func TestOptRoundTrip(t *testing.T) {
	const head = `{"requiredNotNullable":"a","requiredNullable":null`

	tests := []struct {
		json string
		want Opt[string]
	}{
		{head + `}`, Opt[string]{}},
		{head + `,"optionalNullable":null}`, Null[string]()},
		{head + `,"optionalNullable":"x"}`, Some("x")},
	}

	for _, tt := range tests {
		var r Record
		if err := json.Unmarshal([]byte(tt.json), &r); err != nil {
			t.Fatal(err)
		}

		if r.OptionalNullable != tt.want {
			t.Errorf("%s: optionalNullable = %+v, want %+v", tt.json, r.OptionalNullable, tt.want)
		}

		out, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}

		if string(out) != tt.json {
			t.Errorf("round trip of %s = %s", tt.json, out)
		}
	}
}

func TestRequiredNullableIsSent(t *testing.T) {
	out, err := json.Marshal(Record{RequiredNotNullable: "a", RequiredNullable: Ptr("b")})
	if err != nil {
		t.Fatal(err)
	}

	if want := `{"requiredNotNullable":"a","requiredNullable":"b"}`; string(out) != want {
		t.Errorf("Marshal = %s, want %s", out, want)
	}
}
