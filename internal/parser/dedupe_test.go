package parser

import (
	"strings"
	"testing"
)

// TestDeduplicateParams keeps parameters that share a name only once
// camelCased, each under its own name, and lets an operation's parameter
// replace its path item's of the same location and name.
func TestDeduplicateParams(t *testing.T) {
	p := NewParser()

	p.SkipValidation = true
	if err := p.LoadFromData([]byte(`
openapi: 3.0.0
info: {title: t, version: "1"}
paths:
  /things/{item_id}:
    parameters:
      - {name: item_id, in: path, required: true, schema: {type: string}}
    get:
      operationId: getThing
      parameters:
        - {name: item_id, in: path, required: true, schema: {type: integer}}
        - {name: Item-Id, in: header, schema: {type: string}}
        - {name: sort_by, in: query, schema: {type: string}}
        - {name: sortBy, in: query, schema: {type: string}}
      responses: {"204": {description: ok}}
`)); err != nil {
		t.Fatal(err)
	}

	ops, err := p.GetOperations()
	if err != nil {
		t.Fatal(err)
	}

	var got []string
	for _, param := range ops["default"][0].AllParams {
		got = append(got, param.ParamName+"="+param.BaseName+":"+param.DataType)
	}

	if want := "itemId=item_id:number itemId2=Item-Id:string sortBy=sort_by:string sortBy2=sortBy:string"; strings.Join(got, " ") != want {
		t.Errorf("params = %s\nwant     %s", strings.Join(got, " "), want)
	}
}
