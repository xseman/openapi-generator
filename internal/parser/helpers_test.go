package parser

import (
	"testing"

	"github.com/xseman/openapi-generator/internal/codegen"
)

// findModel returns the model named classname, failing the test if it's absent.
func findModel(t *testing.T, models []*codegen.CodegenModel, classname string) *codegen.CodegenModel {
	t.Helper()
	for _, m := range models {
		if m.Classname == classname {
			return m
		}
	}
	t.Fatalf("model %q not found in %d models", classname, len(models))
	return nil
}

func varNames(props []*codegen.CodegenProperty) []string {
	names := make([]string, len(props))
	for i, p := range props {
		names[i] = p.BaseName
	}
	return names
}
