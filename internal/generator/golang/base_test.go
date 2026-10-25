package golang

import (
	"testing"

	"github.com/xseman/openapi-generator/internal/codegen"
)

func TestNames(t *testing.T) {
	g := NewBaseGenerator()

	tests := []struct {
		got, want string
	}{
		{g.ToModelName("pet_id"), "PetID"},
		{g.ToModelName("HTTPGetAction"), "HTTPGetAction"},
		{g.ToModelName("io.k8s.api.v1.Pod"), "IoK8sAPIV1Pod"},
		{g.ToModelName("PET_STATUS"), "PetStatus"},
		{g.ToModelName("File"), "FileModel"},
		{g.ToModelName("Configuration"), "ConfigurationModel"},
		{g.ToModelName("2fa"), "Model2fa"},
		{g.ToVarName("first-name"), "FirstName"},
		{g.ToVarName("last.name"), "LastName"},
		{g.ToVarName("123code"), "Var123code"},
		{g.ToVarName("@type"), "Type"},
		{g.ToVarName("has space"), "HasSpace"},
		{g.ToVarName("a+b"), "AB"},
		{g.ToVarName("_leadingUnderscore"), "LeadingUnderscore"},
		{g.ToVarName("trailingUnderscore_"), "TrailingUnderscore"},
		{g.ToVarName("userUrl"), "UserURL"},
		{g.ToVarName("URLPath"), "URLPath"},
		{g.ToVarName(""), "Var"},
		{g.SanitizeOperationId("findPetById"), "FindPetByID"},
		{g.ToApiName("pet store"), "PetStoreAPI"},
		{g.ToApiFilename("PetStoreAPI"), "api_pet_store"},
		{g.ToApiFilename("LinuxAPI"), "api_linux_"},
		{g.ToModelFilename("HTTPServer"), "model_http_server"},
		{g.ToModelFilename("Test"), "model_test_"},
		{packageName("Pet-Store"), "petstore"},
		{packageName("2pets"), "pets"},
		{packageName("go"), "goapi"},
		{packageName("type"), "typeapi"},
		{packageName("_"), "openapi"},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("got %q, want %q", tt.got, tt.want)
		}
	}
}

func TestGoType(t *testing.T) {
	str := &codegen.CodegenProperty{IsString: true}
	pet := &codegen.CodegenProperty{IsModel: true, DataType: "Pet"}

	tests := []struct {
		prop *codegen.CodegenProperty
		want string
	}{
		{str, "string"},
		{&codegen.CodegenProperty{IsString: true, IsUuid: true}, "string"},
		{&codegen.CodegenProperty{IsString: true, IsDate: true}, "Date"},
		{&codegen.CodegenProperty{IsString: true, IsDateTime: true}, "DateTime"},
		{&codegen.CodegenProperty{IsString: true, IsByteArray: true}, "[]byte"},
		{&codegen.CodegenProperty{IsString: true, IsBinary: true}, "string"},
		{&codegen.CodegenProperty{IsInteger: true}, "int"},
		{&codegen.CodegenProperty{IsInteger: true, IsLong: true}, "int64"},
		{&codegen.CodegenProperty{IsNumber: true}, "float64"},
		{&codegen.CodegenProperty{IsNumber: true, IsFloat: true}, "float32"},
		{&codegen.CodegenProperty{IsNumber: true, IsDouble: true}, "float64"},
		{&codegen.CodegenProperty{IsBoolean: true}, "bool"},
		{&codegen.CodegenProperty{IsArray: true, Items: str}, "[]string"},
		{&codegen.CodegenProperty{IsArray: true, UniqueItems: true, Items: pet}, "[]Pet"},
		{&codegen.CodegenProperty{IsArray: true, Items: &codegen.CodegenProperty{IsArray: true, Items: pet}}, "[][]Pet"},
		{&codegen.CodegenProperty{IsMap: true, Items: pet}, "map[string]Pet"},
		{&codegen.CodegenProperty{IsMap: true}, "map[string]any"},
		{&codegen.CodegenProperty{OpenApiType: "object", IsFreeFormObject: true}, "map[string]any"},
		{&codegen.CodegenProperty{IsAnyType: true}, "any"},
		{pet, "Pet"},
		{&codegen.CodegenProperty{DataType: "Cat | Dog", IsFreeFormObject: true}, "json.RawMessage"},
		{&codegen.CodegenProperty{IsArray: true, Items: &codegen.CodegenProperty{DataType: "Cat | Dog"}}, "[]json.RawMessage"},
	}

	for _, tt := range tests {
		if got := GoType(tt.prop, ""); got != tt.want {
			t.Errorf("GoType(%+v) = %q, want %q", tt.prop, got, tt.want)
		}
	}

	enum := &codegen.CodegenProperty{IsArray: true, IsEnum: true, Items: &codegen.CodegenProperty{IsEnum: true, IsString: true}}
	if got := GoType(enum, "PetTagsEnum"); got != "[]PetTagsEnum" {
		t.Errorf("GoType(array of enum) = %q, want []PetTagsEnum", got)
	}
}

func TestEnumBaseType(t *testing.T) {
	str := []map[string]any{{"isString": true}}

	tests := []struct {
		values []any
		vars   []map[string]any
		want   string
	}{
		{[]any{"a", "b"}, str, "string"},
		{[]any{"a", nil}, str, "string"},
		{[]any{1, 2, nil}, nil, "int"},
		{[]any{"auto", 0}, str, "any"},
		{[]any{1, true}, nil, "any"},
	}

	for _, tt := range tests {
		if got := EnumBaseType(map[string]any{"values": tt.values, "enumVars": tt.vars}, false); got != tt.want {
			t.Errorf("EnumBaseType(%v) = %q, want %q", tt.values, got, tt.want)
		}
	}
}
