package parser

import (
	"reflect"
	"testing"
)

// TestBareMapModelImportsNestedContainerModel guards against a regression where a model
// that is itself a bare map (type: object + additionalProperties, no named properties of
// its own) never imported the model type nested inside its additionalProperties value,
// because schemaToModel only kept the resolved DataType string (discarding the
// CodegenProperty collectImports needs to see) and collectImports never inspected
// model.AdditionalProperties. Without the fix, the generated MapOfArrays.ts types
// `[key: string]: Array<Cell>` but never imports Cell.
func TestBareMapModelImportsNestedContainerModel(t *testing.T) {
	spec := []byte(`
openapi: 3.0.0
info:
  title: Map Model Imports Test
  version: 1.0.0
paths:
  /noop:
    get:
      operationId: noop
      responses:
        '200':
          description: ok
components:
  schemas:
    Cell:
      type: object
      properties:
        value:
          type: string
    MapOfArrays:
      type: object
      additionalProperties:
        type: array
        items:
          $ref: '#/components/schemas/Cell'
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

	mapOfArrays := findModel(t, models, "MapOfArrays")
	if mapOfArrays.AdditionalProperties == nil {
		t.Fatal("MapOfArrays.AdditionalProperties is nil, want a resolved CodegenProperty for Array<Cell>")
	}

	found := false

	for _, imp := range mapOfArrays.Imports {
		if imp == "Cell" {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("MapOfArrays.Imports = %v, want it to contain %q", mapOfArrays.Imports, "Cell")
	}
}

// TestMapOfArraysResponseImportsItemModel guards the return import of a map of arrays:
// { [key: string]: Array<Pet>; } imports Pet, not a non-existent ArrayPet, and a map of
// arrays of primitives or of nested arrays imports nothing and needs no mapper.
func TestMapOfArraysResponseImportsItemModel(t *testing.T) {
	spec := []byte(`
openapi: 3.1.0
info: {title: t, version: "1"}
paths:
  /pets:
    get:
      operationId: getPets
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: {type: object, additionalProperties: {type: array, items: {$ref: "#/components/schemas/Pet"}}}
  /tags:
    get:
      operationId: getTags
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: {type: object, additionalProperties: {type: array, items: {type: string}}}
  /grid:
    get:
      operationId: getGrid
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: {type: object, additionalProperties: {type: array, items: {type: array, items: {$ref: "#/components/schemas/Pet"}}}}
components:
  schemas:
    Pet: {type: object, properties: {name: {type: string}}}
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

	want := map[string][]string{"getPets": {"Pet"}, "getTags": {}, "getGrid": {}}

	for _, group := range ops {
		for _, op := range group {
			if !reflect.DeepEqual(op.Imports, want[op.OperationId]) {
				t.Errorf("%s: Imports = %v, want %v", op.OperationId, op.Imports, want[op.OperationId])
			}

			if wantPrimitive := op.OperationId != "getPets"; op.ReturnTypeIsPrimitive != wantPrimitive {
				t.Errorf("%s: ReturnTypeIsPrimitive = %v, want %v", op.OperationId, op.ReturnTypeIsPrimitive, wantPrimitive)
			}
		}
	}
}
