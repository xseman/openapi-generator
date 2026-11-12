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

	ext := models[2].VendorExtensions
	read, _ := ext["x-dart-read"].([]string)

	var names []string

	variants, _ := ext["x-dart-variants"].([]map[string]any)
	for _, v := range variants {
		name, _ := v["name"].(string)
		names = append(names, name)
	}

	if want := []string{"PetCat", "PetDog", "PetString", "PetDateTime"}; !slices.Equal(names, want) {
		t.Errorf("variants = %v, want %v", names, want)
	}

	body := strings.Join(read, "\n")
	order := []string{
		`switch (json["kind"]) {`, `case "cat":`, `return PetCat(Cat.fromJson(json));`, `case "dog":`,
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

	if ext["x-dart-other"] != "PetOther" {
		t.Errorf("other = %v, want PetOther", ext["x-dart-other"])
	}
}
