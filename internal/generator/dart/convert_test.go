package dart

import (
	"slices"
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
			{Name: "uid", BaseName: "uid", IsString: true, Required: true, IsReadOnly: true},
			{Name: "secret", BaseName: "secret", IsString: true, Required: true, IsWriteOnly: true},
		},
		AdditionalProperties: &codegen.CodegenProperty{IsInteger: true},
	}})

	decl, _ := models[0].VendorExtensions["x-dart-struct"].(map[string]any)
	fields, _ := decl["fields"].([]map[string]any)
	want := [][3]string{
		{"required final int id", `id: (json["id"] as num).toInt()`, `"id": id`},
		{"final DateTime? bornOn", `bornOn: json["born_on"] == null ? null : DateTime.parse(json["born_on"] as String)`, `if (bornOn case final v?) "born_on": formatDate(v)`},
		{"required final String? owner", `owner: json["owner"] == null ? null : json["owner"] as String`, `"owner": owner`},
		{"final Opt<String>? tag", `tag: json.containsKey("tag") ? Opt(json["tag"] == null ? null : json["tag"] as String) : null`, `if (tag case Opt(:final value)) "tag": value`},
		{"final String? uid", `uid: json["uid"] == null ? null : json["uid"] as String`, `"uid": ?uid`},
		{"final String? secret", `secret: json["secret"] == null ? null : json["secret"] as String`, `"secret": ?secret`},
		{
			"final Map<String, int> additionalProperties = const {}",
			`additionalProperties: {for (final e in json.entries) if (!const {"id", "born_on", "owner", "tag", "uid", "secret"}.contains(e.key)) e.key: (e.value as num).toInt()}`,
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

func TestInlineStruct(t *testing.T) {
	g := NewFetchGenerator()
	address := &codegen.CodegenProperty{
		Name: "address", BaseName: "address", OpenApiType: "object",
		Vars: []*codegen.CodegenProperty{{Name: "city", BaseName: "city", IsString: true, Required: true}},
	}
	user := &codegen.CodegenProperty{
		Name: "users", BaseName: "users", IsArray: true,
		Items: &codegen.CodegenProperty{OpenApiType: "object", Vars: []*codegen.CodegenProperty{
			{Name: "name", BaseName: "name", IsString: true, Required: true},
			address,
		}},
	}
	models := g.PostProcessModels([]*codegen.CodegenModel{{Classname: "Team", SchemaName: "Team", AllVars: []*codegen.CodegenProperty{user}}})

	decl, _ := models[0].VendorExtensions["x-dart-struct"].(map[string]any)
	fields, _ := decl["fields"].([]map[string]any)
	structs, _ := models[0].VendorExtensions["x-dart-structs"].([]map[string]any)

	got := []any{fields[0]["decl"], fields[0]["fromJson"], fields[0]["toJson"]}
	want := []any{
		"final List<TeamUsers>? users",
		`users: json["users"] == null ? null : (json["users"] as List<Object?>).map((e) => TeamUsers.fromJson(e as Map<String, Object?>)).toList()`,
		`if (users case final v?) "users": v.map((e) => e.toJson()).toList()`,
	}

	if !slices.Equal(got, want) {
		t.Errorf("users = %q, want %q", got, want)
	}

	var names []any
	for _, s := range structs {
		names = append(names, s["name"])
	}

	if !slices.Equal(names, []any{"TeamUsersAddress", "TeamUsers"}) {
		t.Errorf("inline classes = %v, want TeamUsersAddress, TeamUsers", names)
	}
}
