package parser

import "testing"

// TestArrayModelItemsRef checks that an array schema whose items are a $ref
// types its items as the referenced model, not the model's inline shape.
func TestArrayModelItemsRef(t *testing.T) {
	p := NewParser()

	p.SkipValidation = true
	if err := p.LoadFromData([]byte(`
openapi: 3.0.0
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    Pets: {type: array, items: {$ref: "#/components/schemas/Pet"}}
    Pet: {type: object, properties: {name: {type: string}}}
`)); err != nil {
		t.Fatalf("LoadFromData: %v", err)
	}

	models, err := p.GetModels()
	if err != nil {
		t.Fatalf("GetModels: %v", err)
	}

	if m := findModel(t, models, "Pets"); m.Items == nil || !m.Items.IsModel || m.ArrayModelType != "Pet" {
		t.Errorf("Pets items = %+v, array model type %q, want the Pet model", m.Items, m.ArrayModelType)
	}
}
