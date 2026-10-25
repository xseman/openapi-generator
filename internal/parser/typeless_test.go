package parser

import "testing"

// TestSwagger2TypelessDefinition: openapi2conv leaves an empty properties
// map on every schema, which must not make a typeless one an object (a map
// in go-fetch, which fails on any other JSON value).
func TestSwagger2TypelessDefinition(t *testing.T) {
	spec := []byte(`{
		"swagger": "2.0",
		"info": {"title": "t", "version": "1"},
		"paths": {},
		"definitions": {"JSON": {"description": "any valid JSON value"}}
	}`)

	p := NewParser()

	p.SkipValidation = true
	if err := p.LoadFromData(spec); err != nil {
		t.Fatal(err)
	}

	models, err := p.GetModels()
	if err != nil {
		t.Fatal(err)
	}

	// As the same schema in 3.0: no type, not an object's "any".
	if m := findModel(t, models, "JSON"); m.DataType != "" || m.HasVars {
		t.Errorf("JSON = %q, vars %t; want no type, as in 3.0", m.DataType, m.HasVars)
	}
}
