package gen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTypeScriptFetchContainerSerialization guards two apis.mustache regressions. A map
// of arrays response rendered `runtime.mapValues(jsonValue, Array&lt;Street&gt;FromJSON)`
// instead of mapping each array element. A multipart array of models was csv-joined into
// "[object Object],…" instead of being sent as one JSON part, the way a single model is.
// Arrays of files keep their per-element encoding; arrays of primitives, and of $ref to an
// enum or a primitive schema, stay csv.
func TestTypeScriptFetchContainerSerialization(t *testing.T) {
	spec := writeSpec(t, "spec.yaml", `openapi: 3.1.0
info: {title: t, version: "1"}
paths:
  /streets:
    get:
      operationId: getStreets
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                additionalProperties:
                  type: array
                  items: {$ref: "#/components/schemas/Street"}
  /tags:
    get:
      operationId: getTags
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                additionalProperties:
                  type: array
                  items: {type: string}
  /photos:
    post:
      operationId: storePhotos
      requestBody:
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                meta:
                  type: array
                  items: {$ref: "#/components/schemas/Street"}
                uniqueMeta:
                  type: array
                  uniqueItems: true
                  items: {$ref: "#/components/schemas/Street"}
                tags:
                  type: array
                  items: {type: string}
                colors:
                  type: array
                  items: {$ref: "#/components/schemas/Color"}
                ids:
                  type: array
                  items: {$ref: "#/components/schemas/Id"}
                photos:
                  type: array
                  items: {type: string, format: binary}
      responses:
        "204": {description: ok}
components:
  schemas:
    Color: {type: string, enum: [red, blue]}
    Id: {type: integer}
    Street:
      type: object
      properties:
        name: {type: string}
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
		`runtime.mapValues(jsonValue, (items) => items.map(StreetFromJSON))`,
		`new runtime.JSONApiResponse<{ [key: string]: Array<string>; }>(response)`,
		`formParams.append("meta", new Blob([JSON.stringify(requestParameters["meta"].map(StreetToJSON))], { type: "application/json", }))`,
		`formParams.append("uniqueMeta", new Blob([JSON.stringify(Array.from(requestParameters["uniqueMeta"]).map(StreetToJSON))], { type: "application/json", }))`,
		`formParams.append("tags", requestParameters["tags"]!.join(runtime.COLLECTION_FORMATS["csv"]))`,
		`formParams.append("colors", requestParameters["colors"]!.join(runtime.COLLECTION_FORMATS["csv"]))`,
		`formParams.append("ids", requestParameters["ids"]!.join(runtime.COLLECTION_FORMATS["csv"]))`,
		`formParams.append("photos", element as any)`,
	} {
		if !strings.Contains(api, want) {
			t.Errorf("defaultApi.ts lacks %s", want)
		}
	}

	for _, unwanted := range []string{"ArrayStreet", "ArrayString", "Array&lt;"} {
		if strings.Contains(api, unwanted) {
			t.Errorf("defaultApi.ts contains %q", unwanted)
		}
	}
}
