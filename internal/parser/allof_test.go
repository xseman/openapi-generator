package parser

import "testing"

// TestAllOfRequiresInheritedProperty checks that a property a parent declares
// optional, but the extending model lists in its own required, is redeclared
// on that model as required.
func TestAllOfRequiresInheritedProperty(t *testing.T) {
	spec := []byte(`
openapi: 3.0.0
info:
  title: Inherited required
  version: 1.0.0
paths: {}
components:
  schemas:
    Child:
      type: object
      required: [uuid, own]
      allOf:
        - $ref: '#/components/schemas/Parent'
      properties:
        own: {type: boolean}
    Parent:
      type: object
      properties:
        uuid: {type: string, readOnly: true}
        other: {type: string}
`)

	p := NewParser()

	p.SkipValidation = true
	if err := p.LoadFromData(spec); err != nil {
		t.Fatalf("LoadFromData: %v", err)
	}

	models, err := p.GetModels()
	if err != nil {
		t.Fatalf("GetModels: %v", err)
	}

	var got []string

	for _, v := range findModel(t, models, "Child").RequiredVars {
		got = append(got, v.BaseName)
	}

	if len(got) != 2 || got[0] != "own" || got[1] != "uuid" {
		t.Errorf("Child required vars = %v, want [own uuid]", got)
	}

	// Still read-only: a request body leaves it out rather than demand it.
	if ro := findModel(t, models, "Child").ReadOnlyVars; len(ro) != 1 || ro[0].BaseName != "uuid" {
		t.Errorf("Child read-only vars = %v, want [uuid]", ro)
	}
}
