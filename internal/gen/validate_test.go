package gen

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeSpec(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestValidate(t *testing.T) {
	const valid = `openapi: "3.0.0"
info: {title: t, version: "1"}
paths:
  /pets:
    get:
      operationId: listPets
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Pet"
components:
  schemas:
    Pet: {type: object}
`
	const unused = `openapi: "3.0.0"
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    Zebra: {type: object}
    Apple: {type: string}
`
	const noInfo = `openapi: "3.0.0"
paths: {}
`

	t.Run("valid spec has no issues", func(t *testing.T) {
		res := Validate(writeSpec(t, "valid.yaml", valid))
		if len(res.Errors) != 0 || len(res.Warnings) != 0 {
			t.Fatalf("expected no issues, got %+v", res)
		}
	})

	t.Run("unused models are sorted warnings, not errors", func(t *testing.T) {
		res := Validate(writeSpec(t, "unused.yaml", unused))
		if len(res.Errors) != 0 {
			t.Fatalf("unexpected errors: %v", res.Errors)
		}
		want := []string{"Unused model: Apple", "Unused model: Zebra"}
		if !reflect.DeepEqual(res.Warnings, want) {
			t.Fatalf("warnings = %v, want %v", res.Warnings, want)
		}
	})

	t.Run("invalid spec reports kin-openapi error", func(t *testing.T) {
		res := Validate(writeSpec(t, "noinfo.yaml", noInfo))
		if len(res.Errors) != 1 || !strings.Contains(res.Errors[0], "invalid info") {
			t.Fatalf("errors = %v, want one 'invalid info' error", res.Errors)
		}
	})

	t.Run("unreadable spec is reported as an error", func(t *testing.T) {
		res := Validate(filepath.Join(t.TempDir(), "missing.yaml"))
		if len(res.Errors) != 1 || !strings.Contains(res.Errors[0], "missing.yaml") {
			t.Fatalf("errors = %v, want one load error naming the file", res.Errors)
		}
	})
}
