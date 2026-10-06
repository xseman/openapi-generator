package gen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRequestBodyOmitsReadOnly checks that a request body typed as a model leaves
// its read-only properties out, own or required through allOf, so a caller need
// not send what the server assigns.
func TestRequestBodyOmitsReadOnly(t *testing.T) {
	spec := writeSpec(t, "spec.yaml", `openapi: 3.0.0
info: {title: t, version: "1"}
paths:
  /child:
    post:
      operationId: postChild
      requestBody: {required: true, content: {application/json: {schema: {$ref: '#/components/schemas/Child'}}}}
      responses: {'204': {description: ok}}
  /item:
    post:
      operationId: postItem
      requestBody: {required: true, content: {application/json: {schema: {$ref: '#/components/schemas/Item'}}}}
      responses: {'204': {description: ok}}
components:
  schemas:
    Parent:
      type: object
      properties:
        uuid: {type: string, readOnly: true}
    Child:
      type: object
      required: [uuid, own]
      allOf: [{$ref: '#/components/schemas/Parent'}]
      properties:
        own: {type: boolean}
    Item:
      type: object
      required: [id, name]
      properties:
        id: {type: string, readOnly: true}
        name: {type: string}
`)
	out := t.TempDir()

	if err := Generate(Options{InputSpec: spec, OutputDir: out, GeneratorName: "typescript-fetch"}); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	api, err := os.ReadFile(filepath.Join(out, "apis", "defaultApi.ts"))
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"child: Omit<Child, |'uuid'>;", "item: Omit<Item, |'id'>;"} {
		if !strings.Contains(string(api), want) {
			t.Errorf("apis/defaultApi.ts lacks %q", want)
		}
	}
}
