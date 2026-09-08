package parser

import "testing"

// TestSwagger2PropertyOrderPreservesDeclarationOrder guards against a regression where
// buildPropertyOrder only walked OpenAPI 3.x's "components.schemas" and silently left
// Swagger 2.0 sources ("definitions") falling back to alphabetical property order. Java
// preserves declaration order for both input formats; this spec's properties are
// deliberately declared out of alphabetical order (zebra, apple, mango) so an
// alphabetical fallback would be caught immediately.
func TestSwagger2PropertyOrderPreservesDeclarationOrder(t *testing.T) {
	spec := []byte(`{
		"swagger": "2.0",
		"info": {"title": "Swagger2 Order Test", "version": "1.0.0"},
		"paths": {
			"/noop": {
				"get": {
					"operationId": "noop",
					"responses": {"200": {"description": "ok"}}
				}
			}
		},
		"definitions": {
			"Widget": {
				"type": "object",
				"properties": {
					"zebra": {"type": "string"},
					"apple": {"type": "string"},
					"mango": {"type": "string"}
				}
			}
		}
	}`)

	p := NewParser()
	p.SkipValidation = true
	if err := p.LoadFromData(spec); err != nil {
		t.Fatalf("LoadFromData: %v", err)
	}

	models, err := p.GetModels()
	if err != nil {
		t.Fatalf("GetModels: %v", err)
	}

	widget := findModel(t, models, "Widget")
	got := varNames(widget.Vars)
	want := []string{"zebra", "apple", "mango"}
	if len(got) != len(want) {
		t.Fatalf("Widget.Vars = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Widget.Vars = %v, want %v (declaration order, not alphabetical)", got, want)
		}
	}
}
