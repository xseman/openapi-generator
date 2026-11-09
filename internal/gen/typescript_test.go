package gen

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xseman/openapi-generator/internal/codegen"
	"github.com/xseman/openapi-generator/internal/generator"
)

// TestTypeScriptFetchContainerSerialization guards apis.mustache: a map of arrays response
// maps each element, and a multipart array of models is one JSON part, while arrays of
// files, primitives, enums and primitive aliases keep their per-element and csv encodings.
func TestTypeScriptFetchContainerSerialization(t *testing.T) {
	spec := writeSpec(t, "spec.yaml", `openapi: 3.1.0
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
    post:
      operationId: addPets
      requestBody:
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                pets: {type: array, items: {$ref: "#/components/schemas/Pet"}}
                colors: {type: array, items: {$ref: "#/components/schemas/Color"}}
                ids: {type: array, items: {$ref: "#/components/schemas/Id"}}
                tags: {type: array, items: {type: string}}
                files: {type: array, items: {type: string, format: binary}}
      responses:
        "204": {description: ok}
components:
  schemas:
    Color: {type: string, enum: [red, blue]}
    Id: {type: integer}
    Pet: {type: object, properties: {name: {type: string}}}
`)

	out := t.TempDir()

	err := Generate(Options{
		InputSpec:     spec,
		OutputDir:     out,
		GeneratorName: "typescript-fetch",
		TemplateDir:   filepath.Join("..", "..", "templates", "typescript-fetch"),
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(out, "apis", "defaultApi.ts"))
	if err != nil {
		t.Fatal(err)
	}

	api := string(data)

	for _, want := range []string{
		`runtime.mapValues(jsonValue, (v: any) => (v as Array<any>).map(PetFromJSON))`,
		`formParams.append("pets", new Blob([JSON.stringify((requestParameters["pets"] as Array<any>).map(PetToJSON))], { type: "application/json", }))`,
		`runtime.explodedFields("colors", (requestParameters["colors"] as Array<any>).map(ColorToJSON))`,
		`runtime.explodedFields("ids", (requestParameters["ids"] as Array<any>).map(IdToJSON))`,
		`runtime.explodedFields("tags", requestParameters["tags"])`,
		`formParams.append("files", file)`,
	} {
		if !strings.Contains(api, want) {
			t.Errorf("defaultApi.ts lacks %s", want)
		}
	}

	if strings.Contains(api, "ArrayPet") {
		t.Error("defaultApi.ts imports ArrayPet")
	}
}

// TestTypeScriptFetchScalarParams guards paramText: a path, header or cookie
// value that is surely a scalar is written as its text, like upstream, and
// one that may be a list or an object (a union with an object, an empty
// schema) goes through simpleParam, else it reaches the URL as
// "[object Object]".
func TestTypeScriptFetchScalarParams(t *testing.T) {
	spec := writeSpec(t, "spec.yaml", `openapi: 3.0.3
info: {title: t, version: "1"}
paths:
  /a/{id}/{at}/{union}/{free}:
    get:
      operationId: odd
      parameters:
        - {name: id, in: path, required: true, schema: {type: integer}}
        - {name: at, in: path, required: true, schema: {type: string, format: date-time}}
        - {name: union, in: path, required: true, schema: {$ref: "#/components/schemas/U"}}
        - {name: free, in: path, required: true, schema: {}}
        - {name: h, in: header, schema: {type: string}}
        - {name: c, in: cookie, schema: {$ref: "#/components/schemas/U"}}
      responses:
        "204": {description: ok}
components:
  schemas:
    U: {oneOf: [{type: string}, {$ref: "#/components/schemas/Obj"}]}
    Obj: {type: object, properties: {x: {type: string}}}
`)

	out := t.TempDir()

	err := Generate(Options{
		InputSpec:     spec,
		OutputDir:     out,
		GeneratorName: "typescript-fetch",
		TemplateDir:   "../../templates/typescript-fetch",
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(out, "apis", "defaultApi.ts"))
	if err != nil {
		t.Fatal(err)
	}

	api := string(data)

	for _, want := range []string{
		`encodeURIComponent(String(requestParameters["id"]))`,
		`encodeURIComponent(String(runtime.serializeDateTime(requestParameters["at"])))`,
		`runtime.simpleParam(UToJSON(requestParameters["union"]), false, encodeURIComponent)`,
		`runtime.simpleParam(requestParameters["free"], false, encodeURIComponent)`,
		`headerParameters["h"] = String(requestParameters["h"]);`,
		`cookies.push("c=" + runtime.simpleParam(UToJSON(requestParameters["c"]), true, encodeURIComponent));`,
	} {
		if !strings.Contains(api, want) {
			t.Errorf("defaultApi.ts lacks %s", want)
		}
	}
}

// TestModelRuntimeHelpers guards the runtime import of a model: only the
// helpers its conversions call, by whole name, and not a read-only
// property's ToJSON, which the model leaves out.
func TestModelRuntimeHelpers(t *testing.T) {
	property := func(from, to string, readOnly bool) *codegen.CodegenProperty {
		return &codegen.CodegenProperty{
			IsReadOnly:       readOnly,
			VendorExtensions: map[string]any{"x-ts-from-json": from, "x-ts-to-json": to},
		}
	}

	tests := []struct {
		name  string
		model *generator.CodegenModel
		want  []string
	}{
		{
			name:  "no helper",
			model: &generator.CodegenModel{Vars: []*codegen.CodegenProperty{property(`"a": json["a"]`, `"a": value["a"]`, false)}},
		},
		{
			name: "date-time read-only",
			model: &generator.CodegenModel{Vars: []*codegen.CodegenProperty{
				property(`"at": parseDateTime(json["at"])`, `"at": serializeDateTime(value["at"])`, true),
			}},
			want: []string{"parseDateTime"},
		},
		{
			name: "date apart from date-time",
			model: &generator.CodegenModel{Vars: []*codegen.CodegenProperty{
				property(`"on": (json["on"] as Array<any>).map(parseDate)`, `"on": serializeDate(value["on"])`, false),
			}},
			want: []string{"parseDate", "serializeDate"},
		},
		{
			name: "keys next to the properties",
			model: &generator.CodegenModel{VendorExtensions: map[string]any{
				"x-ts-rest-from": `mapValues(otherKeys(json, ["a"]), parseDateTime)`,
			}},
			want: []string{"mapValues", "otherKeys", "parseDateTime"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := modelRuntimeHelpers(tt.model)

			if !slices.Equal(got, tt.want) {
				t.Errorf("modelRuntimeHelpers() = %v, want %v", got, tt.want)
			}
		})
	}
}
