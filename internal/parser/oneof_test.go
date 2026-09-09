package parser

import "testing"

// TestInlineMapOneOfMemberIsPrimitive guards against a regression where an inline map
// member of a oneOf (type: object + additionalProperties, no $ref) was classified as a
// model: its type declaration ("{ [key: string]: Array<string>; }") landed in
// model.OneOfModels, and modelOneOf.mustache then emitted it as an identifier, producing
// invalid TypeScript such as `instanceOf{ [key: string]: Array<string>; }(json)`. Map
// members belong in OneOfPrimitives, which renders typed JSON conversion branches
// instead of model imports.
func TestInlineMapOneOfMemberIsPrimitive(t *testing.T) {
	spec := []byte(`
openapi: 3.0.0
info:
  title: OneOf Inline Map Test
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
    ValidationErrors:
      oneOf:
        - type: object
          additionalProperties:
            type: array
            items:
              type: string
        - type: array
          maxItems: 0
          items:
            type: string
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

	errs := findModel(t, models, "ValidationErrors")
	if len(errs.OneOfModels) != 0 {
		t.Errorf("ValidationErrors.OneOfModels = %v, want empty: neither member is a model", errs.OneOfModels)
	}

	var mapMember bool
	for _, prop := range errs.OneOfPrimitives {
		if prop.IsMap {
			mapMember = true
		}
	}
	if !mapMember {
		t.Errorf("ValidationErrors.OneOfPrimitives = %v, want it to contain the inline map member", varNames(errs.OneOfPrimitives))
	}
}

// TestParameterEnumVarsCarryIsString guards the contract apis.mustache relies on to quote
// enum values: every enumVar of a string parameter enum must carry isString, in both the
// stringEnums and the const-object rendering. Without it the generated
// `export enum FooAcceptLanguageEnum { EnUs = en_us }` references undefined identifiers.
func TestParameterEnumVarsCarryIsString(t *testing.T) {
	spec := []byte(`
openapi: 3.0.0
info:
  title: Parameter Enum Test
  version: 1.0.0
paths:
  /zones:
    get:
      operationId: listZones
      parameters:
        - name: Accept-Language
          in: header
          schema:
            type: string
            enum: [en_us, sk]
        - name: limit
          in: query
          schema:
            type: integer
            enum: [10, 20]
      responses:
        '200':
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

	want := map[string]bool{"Accept-Language": true, "limit": false}
	seen := map[string]bool{}
	for _, group := range ops {
		for _, op := range group {
			for _, param := range op.AllParams {
				wantIsString, tracked := want[param.BaseName]
				if !tracked {
					continue
				}
				seen[param.BaseName] = true

				enumVars, ok := param.AllowableValues["enumVars"].([]map[string]any)
				if !ok {
					t.Fatalf("param %q: AllowableValues[%q] = %#v, want []map[string]any", param.BaseName, "enumVars", param.AllowableValues["enumVars"])
				}
				if len(enumVars) != 2 {
					t.Fatalf("param %q: got %d enumVars, want 2", param.BaseName, len(enumVars))
				}
				for _, enumVar := range enumVars {
					if enumVar["isString"] != wantIsString {
						t.Errorf("param %q: enumVar %v isString = %v, want %v", param.BaseName, enumVar["name"], enumVar["isString"], wantIsString)
					}
				}
			}
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("parameter %q not found in generated operations", name)
		}
	}
}
