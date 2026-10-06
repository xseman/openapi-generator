package parser

import (
	"slices"
	"testing"
)

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

// TestOneOfNullOnlyWithOneNullableMember checks that null joins a oneOf union
// only when exactly one member is nullable: with none, null matches nothing;
// with two, it matches both, which oneOf rejects.
func TestOneOfNullOnlyWithOneNullableMember(t *testing.T) {
	spec := []byte(`
openapi: 3.0.3
info:
  title: Nullable oneOf
  version: 1.0.0
paths: {}
components:
  schemas:
    One:
      oneOf:
        - {type: string, nullable: true}
        - {type: number}
    None:
      oneOf:
        - {type: string}
        - {type: number}
    Both:
      oneOf:
        - {type: string, nullable: true}
        - {type: number, nullable: true}
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

	for name, want := range map[string]bool{"One": true, "None": false, "Both": false} {
		if got := slices.Contains(findModel(t, models, name).OneOf, "null"); got != want {
			t.Errorf("%s: null in OneOf = %v, want %v", name, got, want)
		}
	}
}

// TestOneOfMemberTagGuards checks what a oneOf's type guards tell members apart
// by: a lone enum value (singleValue, as the template's mustache has no
// -first/-last) and the values a string's `not: {enum}` excludes.
func TestOneOfMemberTagGuards(t *testing.T) {
	spec := []byte(`
openapi: 3.1.0
info:
  title: Tags
  version: 1.0.0
paths: {}
components:
  schemas:
    Known:
      type: object
      required: [kind]
      properties:
        kind: {type: string, enum: [known]}
    Other:
      type: object
      required: [file-path]
      properties:
        file-path:
          type: string
          not: {enum: [known, 'a"b', 42, null]}
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

	single, _ := findModel(t, models, "Known").Vars[0].AllowableValues["singleValue"].([]map[string]any)
	if len(single) != 1 || single[0]["value"] != "known" {
		t.Errorf("Known.kind singleValue = %v, want [{value: known}]", single)
	}

	got := findModel(t, models, "Other").Vars[0].VendorExtensions["x-typescript-fetch-not-enum-comparison"]

	want := `(value as Record<string, unknown>)["filePath"] === "known" || (value as Record<string, unknown>)["file-path"] === "known" || ` +
		`(value as Record<string, unknown>)["filePath"] === "a\"b" || (value as Record<string, unknown>)["file-path"] === "a\"b" || ` +
		`(value as Record<string, unknown>)["filePath"] === null || (value as Record<string, unknown>)["file-path"] === null`
	if got != want {
		t.Errorf("Other.file-path not-enum comparison =\n%v\nwant\n%v", got, want)
	}
}
