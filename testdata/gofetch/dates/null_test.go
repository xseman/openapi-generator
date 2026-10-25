package dates

import (
	"encoding/json"
	"testing"
)

// TestDateNullAndInvalid reads null as the zero Date and refuses a date
// that is not one.
func TestDateNullAndInvalid(t *testing.T) {
	d := Date{}
	if err := json.Unmarshal([]byte("null"), &d); err != nil || !d.IsZero() {
		t.Errorf("null: %v, %v", d, err)
	}

	for _, in := range []string{`"2024-13-01"`, `20240101`} {
		if err := json.Unmarshal([]byte(in), &d); err == nil {
			t.Errorf("%s decoded as %v", in, d)
		}
	}
}
