package parser

import "testing"

// TestModelEnumHasEnumVars guards against top-level enum schemas rendering as an
// empty `export const Status = {} as const`: the enum templates iterate
// allowableValues.enumVars, which only inline (property) enums used to get.
func TestModelEnumHasEnumVars(t *testing.T) {
	spec := []byte(`
openapi: 3.0.0
info:
  title: Enums
  version: 1.0.0
paths: {}
components:
  schemas:
    Status:
      type: string
      nullable: true
      enum: [active, "weird value!", ACTIVE, null]
    Priority:
      type: integer
      enum: [0, 99, -1]
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

	for _, tt := range []struct {
		model    string
		want     []string
		isString bool
	}{
		{"Status", []string{"Active", "WeirdValue", "Active2"}, true},
		{"Priority", []string{"_0", "_99", "Minus1"}, false},
	} {
		vars, _ := findModel(t, models, tt.model).AllowableValues["enumVars"].([]map[string]any)
		if len(vars) != len(tt.want) {
			t.Fatalf("%s enumVars = %v, want names %v", tt.model, vars, tt.want)
		}

		for i, v := range vars {
			if v["name"] != tt.want[i] || v["isString"] != tt.isString {
				t.Errorf("%s enumVars[%d] = %v, want name %s, isString %v", tt.model, i, v, tt.want[i], tt.isString)
			}
		}
	}
}
