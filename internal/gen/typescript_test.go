package gen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		`formParams.append("pets", new Blob([JSON.stringify((requestParameters["pets"]! as Array<any>).map(PetToJSON))], { type: "application/json", }))`,
		`runtime.formFields("colors", (requestParameters["colors"]! as Array<any>).map(ColorToJSON), "form", true)`,
		`runtime.formFields("ids", (requestParameters["ids"]! as Array<any>).map(IdToJSON), "form", true)`,
		`runtime.formFields("tags", requestParameters["tags"]!, "form", true)`,
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
