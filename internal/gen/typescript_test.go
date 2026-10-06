package gen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTypeScriptFetchContainerSerialization guards an apis.mustache regression: a map of
// arrays response rendered `runtime.mapValues(jsonValue, Array&lt;Street&gt;FromJSON)`
// instead of mapping each array element.
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
components:
  schemas:
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
