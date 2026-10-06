package parser

import "testing"

// TestQueryParamExplodeDefaults checks the style/explode flags the query template
// branches on: form style explodes unless told otherwise, deepObject is flagged,
// and a free-form object (but not a typeless oneOf) counts as a map, as upstream.
func TestQueryParamExplodeDefaults(t *testing.T) {
	spec := []byte(`
openapi: 3.0.0
info:
  title: Explode
  version: 1.0.0
paths:
  /q:
    get:
      operationId: getQ
      parameters:
        - {name: labels, in: query, explode: false, schema: {type: object, additionalProperties: {type: string}}}
        - {name: extra, in: query, schema: {type: object}}
        - {name: range, in: query, style: deepObject, explode: true, schema: {type: object, additionalProperties: {type: integer}}}
        - {name: has, in: query, schema: {oneOf: [{type: string}, {type: array, items: {type: string}}]}}
      responses:
        '204':
          description: ok
`)

	p := NewParser()

	p.SkipValidation = true
	if err := p.LoadFromData(spec); err != nil {
		t.Fatalf("LoadFromData: %v", err)
	}

	ops, err := p.GetOperations()
	if err != nil {
		t.Fatalf("GetOperations: %v", err)
	}

	want := map[string][3]bool{ // explode, deepObject, map
		"labels": {false, false, true},
		"extra":  {true, false, true},
		"range":  {true, true, true},
		"has":    {true, false, false}, // a oneOf is no map, though it has no type
	}

	for _, list := range ops {
		for _, op := range list {
			for _, qp := range op.QueryParams {
				got := [3]bool{qp.IsExplode, qp.IsDeepObject, qp.IsMap}
				if got != want[qp.BaseName] {
					t.Errorf("%s: explode, deepObject, map = %v, want %v", qp.BaseName, got, want[qp.BaseName])
				}
			}
		}
	}
}
