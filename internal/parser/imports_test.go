package parser

import (
	"reflect"
	"testing"

	"github.com/xseman/openapi-generator/internal/codegen"
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

// TestMapOfArraysResponseImportsItemModel guards against a regression where a response
// typed as a map of arrays ({ [key: string]: Array<Street>; }) imported the type
// declaration "Array<Street>" as if it were a model, so the generated API imported a
// non-existent ArrayStreet. The import must be the item model; a map of arrays of
// primitives or of nested containers imports nothing and is passed through unmapped.
func TestMapOfArraysResponseImportsItemModel(t *testing.T) {
	spec := []byte(`
openapi: 3.1.0
info:
  title: Map Of Arrays Response Test
  version: 1.0.0
paths:
  /streets:
    get:
      operationId: getStreets
      responses:
        '200':
          description: ok
          content:
            application/json:
              schema:
                type: object
                additionalProperties:
                  type: array
                  uniqueItems: true
                  items:
                    $ref: '#/components/schemas/Street'
  /tags:
    get:
      operationId: getTags
      responses:
        '200':
          description: ok
          content:
            application/json:
              schema:
                type: object
                additionalProperties:
                  type: array
                  items:
                    type: string
  /grid:
    get:
      operationId: getGrid
      responses:
        '200':
          description: ok
          content:
            application/json:
              schema:
                type: object
                additionalProperties:
                  type: array
                  items:
                    type: array
                    items:
                      $ref: '#/components/schemas/Street'
components:
  schemas:
    Street:
      type: object
      properties:
        name:
          type: string
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

	tests := []struct {
		operationID   string
		wantImports   []string
		wantPrimitive bool
	}{
		{operationID: "getStreets", wantImports: []string{"Street"}, wantPrimitive: false},
		{operationID: "getTags", wantImports: []string{}, wantPrimitive: true},
		{operationID: "getGrid", wantImports: []string{}, wantPrimitive: true},
	}

	for _, tt := range tests {
		t.Run(tt.operationID, func(t *testing.T) {
			var op *codegen.CodegenOperation

			for _, group := range ops {
				for _, o := range group {
					if o.OperationId == tt.operationID {
						op = o
					}
				}
			}

			if op == nil {
				t.Fatalf("operation %q not found", tt.operationID)
			}

			if !reflect.DeepEqual(op.Imports, tt.wantImports) {
				t.Errorf("Imports = %v, want %v", op.Imports, tt.wantImports)
			}

			if op.ReturnTypeIsPrimitive != tt.wantPrimitive {
				t.Errorf("ReturnTypeIsPrimitive = %v, want %v", op.ReturnTypeIsPrimitive, tt.wantPrimitive)
			}

			if op.ReturnProperty == nil || op.ReturnProperty.Items == nil || !op.ReturnProperty.Items.IsArray {
				t.Errorf("ReturnProperty = %+v, want a map whose items are an array", op.ReturnProperty)
			}
		})
	}
}
