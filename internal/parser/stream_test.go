package parser

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/xseman/openapi-generator/internal/codegen"
)

// TestStreamItems: a response with only an itemSchema validates in a 3.1
// document, as FastAPI writes one, and records its item: of a server-sent
// event, its data's contentSchema.
func TestStreamItems(t *testing.T) {
	p := NewParser()
	if err := p.LoadFromFile(filepath.Join("..", "..", "testdata", "specs", "feature", "responses-stream.yaml")); err != nil {
		t.Fatal(err)
	}

	ops, err := p.GetOperations()
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"streamTicks": "application/jsonl Tick", "streamWords": "application/jsonl string", "chat": "text/event-stream union", "rawEvents": "text/event-stream string",
		"pings": "text/event-stream string", "records": "application/json-seq Tick",
		"maybeTicks": "text/event-stream Tick nullable",
	}

	for _, op := range ops["default"] {
		r := op.Responses[0]
		mediaType, _ := r.VendorExtensions["x-stream"].(string)
		item, _ := r.VendorExtensions["x-stream-item"].(*codegen.CodegenProperty)

		got := "none"
		if item != nil {
			got = mediaType + " " + item.DataType
			if item.IsNullable {
				got += " nullable"
			}

			if item.ComposedSchemas != nil {
				got = mediaType + " union"
			}
		}

		if got != want[op.OperationId] || op.ReturnType != "" {
			t.Errorf("%s = %s, returns %q; want %s and no return type", op.OperationId, got, op.ReturnType, want[op.OperationId])
		}
	}
}

// TestOpenAPI32Operations: a 3.1 document may hold 3.2's itemSchema, but not
// the operations 3.2 adds, which no generator reads.
func TestOpenAPI32Operations(t *testing.T) {
	err := NewParser().LoadFromData([]byte(`openapi: 3.1.0
info: {title: t, version: "1"}
paths:
  /search:
    query:
      operationId: search
      responses: {"200": {description: ok}}
`))
	if err == nil || !strings.Contains(err.Error(), "path /search: query and additionalOperations are for OpenAPI >=3.2") {
		t.Errorf("err = %v, want the query operation refused", err)
	}
}
