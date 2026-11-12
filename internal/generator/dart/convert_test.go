package dart

import (
	"testing"

	"github.com/xseman/openapi-generator/internal/codegen"
)

func TestConvert(t *testing.T) {
	g := NewFetchGenerator()
	g.PostProcessModels([]*codegen.CodegenModel{
		{Classname: "Pet", SchemaName: "Pet", AllVars: []*codegen.CodegenProperty{{Name: "name", BaseName: "name", IsString: true}}},
		{Classname: "Status", SchemaName: "Status", IsEnum: true, AllowableValues: map[string]any{
			"values": []any{"on"}, "enumVars": []map[string]any{{"name": "On", "value": "on", "isString": true}},
		}},
		{Classname: "Day", SchemaName: "Day", IsString: true, IsDate: true},
		{Classname: "Pets", SchemaName: "Pets", IsArray: true, Items: &codegen.CodegenProperty{IsModel: true, DataType: "Pet"}},
	})

	pet := &codegen.CodegenProperty{IsModel: true, DataType: "Pet"}
	when := &codegen.CodegenProperty{IsString: true, IsDateTime: true}

	tests := []struct {
		name     string
		prop     *codegen.CodegenProperty
		typ      string
		from, to string
	}{
		{"string", &codegen.CodegenProperty{IsString: true}, "String", "x as String", "x"},
		{"integer", &codegen.CodegenProperty{IsInteger: true}, "int", "(x as num).toInt()", "x"},
		{"model", pet, "Pet", "Pet.fromJson(x as Map<String, Object?>)", "x.toJson()"},
		{"$ref to an enum", &codegen.CodegenProperty{Ref: "#/components/schemas/Status", IsEnum: true, IsString: true}, "Status", "Status.fromJson(x)", "x.toJson()"},
		{"$ref to a date alias", &codegen.CodegenProperty{IsModel: true, DataType: "Day"}, "Day", "DateTime.parse(x as String)", "formatDate(x)"},
		{
			"$ref to a list alias", &codegen.CodegenProperty{IsModel: true, DataType: "Pets"}, "Pets",
			"(x as List<Object?>).map((e) => Pet.fromJson(e as Map<String, Object?>)).toList()", "x.map((e) => e.toJson()).toList()",
		},
		{
			"map of date-times", &codegen.CodegenProperty{IsMap: true, Items: when}, "Map<String, DateTime>",
			"(x as Map<String, Object?>).map((k, v) => MapEntry(k, DateTime.parse(v as String)))",
			"x.map((k, v) => MapEntry(k, v.toUtc().toIso8601String()))",
		},
		{
			"list of nullable models", &codegen.CodegenProperty{IsArray: true, Items: &codegen.CodegenProperty{IsModel: true, DataType: "Pet", IsNullable: true}}, "List<Pet?>",
			"(x as List<Object?>).map((e) => e == null ? null : Pet.fromJson(e as Map<String, Object?>)).toList()",
			"x.map((e) => e?.toJson()).toList()",
		},
		{"list of strings stays as it is", &codegen.CodegenProperty{IsArray: true, Items: &codegen.CodegenProperty{IsString: true}}, "List<String>", "(x as List<Object?>).map((e) => e as String).toList()", "x"},
	}

	for _, tt := range tests {
		if got := g.typeOf(tt.prop, ""); got != tt.typ {
			t.Errorf("%s: typeOf = %s, want %s", tt.name, got, tt.typ)
		}

		if got := g.fromJSON(tt.prop, "x", ""); got != tt.from {
			t.Errorf("%s: fromJSON = %s, want %s", tt.name, got, tt.from)
		}

		if got := g.toJSON(tt.prop, "x", ""); got != tt.to {
			t.Errorf("%s: toJSON = %s, want %s", tt.name, got, tt.to)
		}
	}
}

// An optional property reads null for a missing key and is left out of the
// JSON when null; a required nullable one is written as null; an optional
// nullable one is an Opt, which tells the two apart; undeclared keys have a
// map of their own.
func TestStructFields(t *testing.T) {
	g := NewFetchGenerator()
	models := g.PostProcessModels([]*codegen.CodegenModel{{
		Classname: "Pet", SchemaName: "Pet",
		AllVars: []*codegen.CodegenProperty{
			{Name: "id", BaseName: "id", IsInteger: true, Required: true},
			{Name: "bornOn", BaseName: "born_on", IsString: true, IsDate: true},
			{Name: "owner", BaseName: "owner", IsString: true, Required: true, IsNullable: true},
			{Name: "tag", BaseName: "tag", IsString: true, IsNullable: true},
		},
		AdditionalProperties: &codegen.CodegenProperty{IsInteger: true},
	}})

	fields, _ := models[0].VendorExtensions["x-dart-fields"].([]map[string]any)
	want := [][3]string{
		{"required final int id", `id: (json["id"] as num).toInt()`, `"id": id`},
		{"final DateTime? bornOn", `bornOn: json["born_on"] == null ? null : DateTime.parse(json["born_on"] as String)`, `if (bornOn case final v?) "born_on": formatDate(v)`},
		{"required final String? owner", `owner: json["owner"] == null ? null : json["owner"] as String`, `"owner": owner`},
		{"final Opt<String>? tag", `tag: json.containsKey("tag") ? Opt(json["tag"] == null ? null : json["tag"] as String) : null`, `if (tag case Opt(:final value)) "tag": value`},
		{
			"final Map<String, int> additionalProperties = const {}",
			`additionalProperties: {for (final e in json.entries) if (!const {"id", "born_on", "owner", "tag"}.contains(e.key)) e.key: (e.value as num).toInt()}`,
			"...additionalProperties",
		},
	}

	if len(fields) != len(want) {
		t.Fatalf("fields = %v, want %d", fields, len(want))
	}

	for i, w := range want {
		for j, key := range []string{"decl", "fromJson", "toJson"} {
			if got := fields[i][key]; got != w[j] {
				t.Errorf("field %d %s = %v, want %s", i, key, got, w[j])
			}
		}
	}
}
