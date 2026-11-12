package dart

import (
	"testing"

	"github.com/xseman/openapi-generator/internal/codegen"
)

func TestDartType(t *testing.T) {
	text := &codegen.CodegenProperty{IsString: true}
	maybe := &codegen.CodegenProperty{IsInteger: true, IsNullable: true}

	tests := []struct {
		name  string
		prop  *codegen.CodegenProperty
		named string
		want  string
	}{
		{"nothing", nil, "", "Object?"},
		{"string", text, "", "String"},
		{"date", &codegen.CodegenProperty{IsString: true, IsDate: true}, "", "DateTime"},
		{"integer", &codegen.CodegenProperty{IsInteger: true}, "", "int"},
		{"int64", &codegen.CodegenProperty{IsLong: true}, "", "int"},
		{"number", &codegen.CodegenProperty{IsNumber: true}, "", "double"},
		{"free-form object", &codegen.CodegenProperty{OpenApiType: "object"}, "", "Map<String, Object?>"},
		{"inline union", &codegen.CodegenProperty{DataType: "string | number"}, "", "Object?"},
		{"named leaf", &codegen.CodegenProperty{IsModel: true, DataType: "Pet"}, "Pet", "Pet"},
		{"list of nullable items", &codegen.CodegenProperty{IsArray: true, Items: maybe}, "", "List<int?>"},
		{"map of lists of a model", &codegen.CodegenProperty{IsMap: true, Items: &codegen.CodegenProperty{IsArray: true, Items: &codegen.CodegenProperty{IsModel: true}}}, "Pet", "Map<String, List<Pet>>"},
		{"list without items", &codegen.CodegenProperty{IsArray: true}, "", "List<Object?>"},
	}

	for _, tt := range tests {
		if got := DartType(tt.prop, tt.named); got != tt.want {
			t.Errorf("%s: DartType = %s, want %s", tt.name, got, tt.want)
		}
	}
}

func TestNullable(t *testing.T) {
	for in, want := range map[string]string{"int": "int?", "int?": "int?", "Object?": "Object?", "List<int?>": "List<int?>?"} {
		if got := Nullable(in); got != want {
			t.Errorf("Nullable(%s) = %s, want %s", in, got, want)
		}
	}
}

func TestEnumBaseType(t *testing.T) {
	tests := []struct {
		name      string
		values    []any
		isString  bool
		isBoolean bool
		want      string
	}{
		{"strings", []any{"a", "b"}, true, false, "String"},
		{"whole numbers", []any{float64(1), float64(2)}, false, false, "int"},
		{"fractions", []any{0.5, float64(1)}, false, false, "double"},
		{"booleans", []any{true}, false, true, "bool"},
		{"mixed", []any{"auto", float64(0)}, true, false, "Object?"},
		{"null is no type of its own", []any{"a", nil}, true, false, "String"},
	}

	for _, tt := range tests {
		allowable := map[string]any{"values": tt.values, "enumVars": []map[string]any{{"isString": tt.isString}}}
		if got := EnumBaseType(allowable, tt.isBoolean); got != tt.want {
			t.Errorf("%s: EnumBaseType = %s, want %s", tt.name, got, tt.want)
		}
	}
}

func TestLiteral(t *testing.T) {
	if got, want := Literal("it's $5\n"), `"it's \$5\n"`; got != want {
		t.Errorf("Literal = %s, want %s", got, want)
	}
}
