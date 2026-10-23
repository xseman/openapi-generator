package parser

import "testing"

// TestExclusiveBounds checks both spellings of an exclusive bound reach the
// validation attributes: OAS 3.0's boolean beside minimum/maximum, and OAS 3.1's
// own number, which stands in for the limit.
func TestExclusiveBounds(t *testing.T) {
	for _, tt := range []struct {
		version, size string
	}{
		{"3.0.3", "{type: integer, minimum: 0, exclusiveMinimum: true, maximum: 10}"},
		{"3.1.0", "{type: integer, exclusiveMinimum: 0, maximum: 10}"},
	} {
		spec := []byte(`
openapi: ` + tt.version + `
info:
  title: Bounds
  version: 1.0.0
paths: {}
components:
  schemas:
    Box:
      type: object
      properties:
        size: ` + tt.size + `
`)

		p := NewParser()

		p.SkipValidation = true
		if err := p.LoadFromData(spec); err != nil {
			t.Fatalf("%s: LoadFromData: %v", tt.version, err)
		}

		models, err := p.GetModels()
		if err != nil {
			t.Fatalf("%s: GetModels: %v", tt.version, err)
		}

		size := findModel(t, models, "Box").Vars[0]

		got := [4]any{size.Minimum, size.ExclusiveMinimum, size.Maximum, size.ExclusiveMaximum}
		if want := [4]any{"0", true, "10", false}; got != want {
			t.Errorf("%s: minimum, exclusive, maximum, exclusive = %v, want %v", tt.version, got, want)
		}
	}
}
