package typescript

import (
	"testing"

	"github.com/xseman/openapi-generator/internal/codegen"
)

// TestConvert pins the expressions that convert a value from its JSON and
// back: containers at any depth, dates, models, a $ref to a union model, an
// inline union told apart by a pinned property or by a date's text.
func TestConvert(t *testing.T) {
	g := NewFetchGenerator()

	pinnedTo := func(name, value string) *codegen.CodegenModel {
		return &codegen.CodegenModel{Classname: name, Vars: []*codegen.CodegenProperty{{
			Name: "kind", BaseName: "kind", IsEnum: true, Required: true,
			AllowableValues: map[string]any{"values": []any{value}},
		}}}
	}

	g.byName = map[string]*codegen.CodegenModel{
		"Cell":  {Classname: "Cell"},
		"Pet":   {Classname: "Pet", HasOneOf: true},
		"Delta": pinnedTo("Delta", "delta"),
		"Done":  pinnedTo("Done", "done"),
		"Customer": {Classname: "Customer", Vars: []*codegen.CodegenProperty{
			{Name: "taxId", BaseName: "tax_id", Required: true}, {Name: "email", BaseName: "email"},
		}},
	}

	cell := &codegen.CodegenProperty{Datatype: "Cell", IsModel: true}
	day := &codegen.CodegenProperty{IsDateType: true}
	instant := &codegen.CodegenProperty{IsDateTimeType: true}
	array := func(items *codegen.CodegenProperty) *codegen.CodegenProperty {
		return &codegen.CodegenProperty{IsArray: true, IsContainer: true, Items: items}
	}
	union := func(members ...*codegen.CodegenProperty) *codegen.CodegenProperty {
		return &codegen.CodegenProperty{ComposedSchemas: &codegen.CodegenComposedSchemas{OneOf: members}}
	}

	tests := []struct {
		name     string
		p        *codegen.CodegenProperty
		from, to string
	}{
		{"scalar", &codegen.CodegenProperty{Datatype: "string", IsPrimitiveType: true}, "x", "x"},
		{"model", cell, "CellFromJSON(x)", "CellToJSON(x)"},
		{
			"rows of models", array(array(cell)),
			"(x as Array<any>).map((v: any) => (v as Array<any>).map(CellFromJSON))",
			"(x as Array<any>).map((v: any) => (v as Array<any>).map(CellToJSON))",
		},
		{
			"map of instants", &codegen.CodegenProperty{IsMap: true, IsContainer: true, Items: instant},
			"mapValues(x, parseDateTime)", "mapValues(x, serializeDateTime)",
		},
		{
			"days, null among them", array(&codegen.CodegenProperty{IsDateType: true, IsNullable: true}),
			"(x as Array<any>).map((v: any) => v == null ? v : parseDate(v))",
			"(x as Array<any>).map((v: any) => v == null ? v : serializeDate(v))",
		},
		{"$ref to a union", &codegen.CodegenProperty{Ref: "#/components/schemas/Pet", Datatype: "Cat | Dog"}, "PetFromJSON(x)", "PetToJSON(x)"},
		{
			"union by a pinned property",
			union(&codegen.CodegenProperty{Datatype: "Delta", IsModel: true}, &codegen.CodegenProperty{Datatype: "Done", IsModel: true}),
			`discriminate(x, "kind", { "delta": DeltaFromJSON, "done": DoneFromJSON })`,
			`discriminate(x, "kind", { "delta": DeltaToJSON, "done": DoneToJSON })`,
		},
		{
			"union of a day and a number", union(day, &codegen.CodegenProperty{Datatype: "number"}),
			`(typeof x === "string" && /^\d{4}-\d{2}-\d{2}$/.test(x) ? parseDate(x) : x)`,
			"(x instanceof Date ? serializeDate(x) : x)",
		},
		{
			"an id or the object it expands to",
			union(&codegen.CodegenProperty{Datatype: "string", IsPrimitiveType: true}, &codegen.CodegenProperty{Datatype: "Customer", IsModel: true}),
			`bestFit(x, [[["tax_id"], ["tax_id", "email"], CustomerFromJSON]]) ?? x`,
			`bestFit(x, [[["taxId"], ["taxId", "email"], CustomerToJSON]]) ?? x`,
		},
		{"union with a member of no known kind", union(cell, &codegen.CodegenProperty{Datatype: "Array<number>", IsArray: true}), "x", "x"},
	}

	for _, tt := range tests {
		from := (&converter{g: g, used: map[string]bool{}}).expr(tt.p, "x")
		to := (&converter{g: g, to: true, used: map[string]bool{}}).expr(tt.p, "x")

		if from != tt.from || to != tt.to {
			t.Errorf("%s:\nfrom %s\nwant %s\nto   %s\nwant %s", tt.name, from, tt.from, to, tt.to)
		}
	}
}

// TestModelProperties pins a property's value in FromJSON and ToJSON:
// absent and null as the property allows, keys every object inherits.
func TestModelProperties(t *testing.T) {
	g := NewFetchGenerator()
	g.byName = map[string]*codegen.CodegenModel{}

	tags := func(required, nullable bool) *codegen.CodegenProperty {
		return &codegen.CodegenProperty{
			Name: "tags", BaseName: "tags", Required: required, IsNullable: nullable,
			IsArray: true, IsContainer: true, Items: &codegen.CodegenProperty{IsDateType: true},
		}
	}

	tests := []struct {
		name     string
		p        *codegen.CodegenProperty
		from, to string
	}{
		{
			"optional", tags(false, false),
			`json["tags"] == null ? undefined : (json["tags"] as Array<any>).map(parseDate)`,
			`value["tags"] == null ? undefined : (value["tags"] as Array<any>).map(serializeDate)`,
		},
		{
			"optional and nullable: null stays", tags(false, true),
			`json["tags"] === undefined ? undefined : json["tags"] === null ? null : (json["tags"] as Array<any>).map(parseDate)`,
			`value["tags"] == null ? value["tags"] : (value["tags"] as Array<any>).map(serializeDate)`,
		},
		{
			"required", tags(true, false),
			`json["tags"] == null ? json["tags"] : (json["tags"] as Array<any>).map(parseDate)`,
			`value["tags"] == null ? value["tags"] : (value["tags"] as Array<any>).map(serializeDate)`,
		},
		{
			"inherited key", &codegen.CodegenProperty{Name: "_constructor", BaseName: "constructor", Datatype: "string"},
			`(Object.prototype.hasOwnProperty.call(json, "constructor") ? json["constructor"] : undefined) == null ? undefined : ` +
				`(Object.prototype.hasOwnProperty.call(json, "constructor") ? json["constructor"] : undefined)`,
			`value["_constructor"]`,
		},
	}

	for _, tt := range tests {
		from := (&converter{g: g, used: map[string]bool{}}).varFromJSON(tt.p)
		to := (&converter{g: g, to: true, used: map[string]bool{}}).varToJSON(tt.p)

		if from != tt.from || to != tt.to {
			t.Errorf("%s:\nfrom %s\nwant %s\nto   %s\nwant %s", tt.name, from, tt.from, to, tt.to)
		}
	}

	if got := key("__proto__"); got != `["__proto__"]` {
		t.Errorf(`key("__proto__") = %s, want a computed key`, got)
	}
}
