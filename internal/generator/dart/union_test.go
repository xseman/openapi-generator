package dart

import (
	"slices"
	"strings"
	"testing"

	"github.com/xseman/openapi-generator/internal/codegen"
)

// A union reads its value by the property every struct member pins, then by
// JSON type, a date before a string; each member is a variant class.
func TestUnion(t *testing.T) {
	kind := func(value string) *codegen.CodegenProperty {
		return &codegen.CodegenProperty{Name: "kind", BaseName: "kind", IsString: true, IsEnum: true, Required: true, AllowableValues: map[string]any{
			"values": []any{value}, "enumVars": []map[string]any{{"name": value, "value": value, "isString": true}},
		}}
	}

	g := NewFetchGenerator()
	models := g.PostProcessModels([]*codegen.CodegenModel{
		{Classname: "Cat", SchemaName: "Cat", AllVars: []*codegen.CodegenProperty{kind("cat")}},
		{Classname: "Dog", SchemaName: "Dog", AllVars: []*codegen.CodegenProperty{kind("dog"), {Name: "pack", BaseName: "pack", IsInteger: true}}},
		{
			Classname: "Pet", SchemaName: "Pet", OneOf: []string{"Cat", "Dog"}, OneOfModels: []string{"Cat", "Dog"},
			OneOfPrimitives: []*codegen.CodegenProperty{{IsString: true}, {IsString: true, IsDateTime: true}},
		},
	})

	unions, _ := models[2].VendorExtensions["x-dart-unions"].([]map[string]any)
	if len(unions) != 1 {
		t.Fatalf("unions = %v, want one", unions)
	}

	ext := unions[0]
	read, _ := ext["read"].([]string)

	var names []string

	variants, _ := ext["variants"].([]map[string]any)
	for _, v := range variants {
		name, _ := v["name"].(string)
		names = append(names, name)
	}

	if want := []string{"PetCat", "PetDog", "PetString", "PetDateTime"}; !slices.Equal(names, want) {
		t.Errorf("variants = %v, want %v", names, want)
	}

	body := strings.Join(read, "\n")
	order := []string{
		`switch (json["kind"]) {`, `case "cat" when const ["kind"].every(json.containsKey):`, `return PetCat(Cat.fromJson(json));`, `case "dog" when`,
		`if (json is String && DateTime.tryParse(json) != null) return PetDateTime(DateTime.parse(json));`,
		`if (json is String) return PetString(json);`,
		`switch (bestFit(json, const [(["kind"], ["kind"]), (["kind"], ["kind", "pack"])])) {`,
	}

	at := 0

	for _, line := range order {
		i := strings.Index(body[at:], line)
		if i < 0 {
			t.Fatalf("fromJson lacks %q after offset %d:\n%s", line, at, body)
		}

		at += i
	}

	if ext["other"] != "PetOther" {
		t.Errorf("other = %v, want PetOther", ext["other"])
	}
}

// A oneOf a property lists inline is a union named after the model and the
// property, declared with the model; a number in it stays a num.
func TestInlineUnion(t *testing.T) {
	g := NewFetchGenerator()
	models := g.PostProcessModels([]*codegen.CodegenModel{{
		Classname: "Item", SchemaName: "Item",
		AllVars: []*codegen.CodegenProperty{{
			Name: "price", BaseName: "price", DataType: "number | string", Required: true,
			ComposedSchemas: &codegen.CodegenComposedSchemas{OneOf: []*codegen.CodegenProperty{{IsNumber: true}, {IsString: true}}},
		}},
	}})

	ext := models[0].VendorExtensions
	decl, _ := ext["x-dart-struct"].(map[string]any)
	fields, _ := decl["fields"].([]map[string]any)
	unions, _ := ext["x-dart-unions"].([]map[string]any)

	if len(fields) != 1 || len(unions) != 1 {
		t.Fatalf("fields = %v, unions = %v, want one of each", fields, unions)
	}

	if got, want := fields[0]["decl"], "required final ItemPrice price"; got != want {
		t.Errorf("decl = %v, want %s", got, want)
	}

	if got, want := fields[0]["fromJson"], `price: ItemPrice.fromJson(json["price"])`; got != want {
		t.Errorf("fromJson = %v, want %s", got, want)
	}

	read, _ := unions[0]["read"].([]string)
	if body := strings.Join(read, "\n"); !strings.Contains(body, "if (json is num) return ItemPriceNum(json);") || !strings.Contains(body, "if (json is String) return ItemPriceString(json);") {
		t.Errorf("fromJson reads:\n%s", body)
	}
}
