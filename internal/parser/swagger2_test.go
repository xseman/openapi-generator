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

// TestSwagger2YAML: a Swagger 2.0 document in YAML is one too, unquoted
// status codes included; it was read as an OpenAPI 3 one without openapi.
func TestSwagger2YAML(t *testing.T) {
	p := NewParser()

	p.SkipValidation = true
	if err := p.LoadFromData([]byte(`swagger: "2.0"
info: {title: t, version: "1"}
paths:
  /pets:
    get:
      operationId: listPets
      responses:
        200: {description: ok, schema: {$ref: "#/definitions/Pet"}}
definitions:
  Pet: {type: object, properties: {zebra: {type: string}, apple: {type: string}}}
`)); err != nil {
		t.Fatal(err)
	}

	models, err := p.GetModels()
	if err != nil {
		t.Fatal(err)
	}

	if got := varNames(findModel(t, models, "Pet").Vars); len(got) != 2 || got[0] != "zebra" {
		t.Errorf("Pet vars = %v, want [zebra apple]", got)
	}

	ops, err := p.GetOperations()
	if err != nil {
		t.Fatal(err)
	}

	if op := ops["default"]; len(op) != 1 || op[0].ReturnType != "Pet" {
		t.Errorf("operations = %v, want listPets returning Pet", op)
	}
}
