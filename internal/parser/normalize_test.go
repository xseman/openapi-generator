package parser

import (
	"fmt"
	"path/filepath"
	"testing"
)

// TestNormalize31 checks that the 3.1 spellings of null and of a binary
// string read as their 3.0 shapes: the member's type, nullable.
func TestNormalize31(t *testing.T) {
	p := NewParser()

	p.SkipValidation = true
	if err := p.LoadFromFile(filepath.Join("..", "..", "testdata", "specs", "feature", "nullable-openapi31.yaml")); err != nil {
		t.Fatal(err)
	}

	models, err := p.GetModels()
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]string

	for _, m := range models {
		if m.Classname != "Pet" {
			continue
		}

		got = make(map[string]string)
		for _, v := range m.Vars {
			got[v.BaseName] = fmt.Sprintf("%s nullable=%t ref=%s %s", v.DataType, v.IsNullable, v.Ref, v.Description)
		}

		// A const is an enum of it; a choice of consts one enum of them all.
		for _, v := range m.Vars {
			want := map[string]string{"kind": "[pet]", "mood": "[happy sad]"}[v.BaseName]
			if got := fmt.Sprint(v.AllowableValues["values"]); want != "" && (!v.IsEnum || got != want) {
				t.Errorf("%s = enum %t of %s, want %s", v.BaseName, v.IsEnum, got, want)
			}
		}
	}

	want := map[string]string{
		"kind":     "string nullable=false ref= ",
		"name":     "string nullable=false ref= ",
		"size":     "Size nullable=true ref=#/components/schemas/Size ",
		"owner":    "Owner nullable=true ref=#/components/schemas/Owner Who keeps the pet.",
		"born":     "Date nullable=true ref= ",
		"nickname": "string nullable=true ref= ",
		"tags":     "Array<string> nullable=true ref= ",
		"alias":    "string nullable=true ref=#/components/schemas/MaybeName ",
		"mood":     "string nullable=false ref= ",
		"tag":      "string nullable=true ref= ",
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s = %q, want %q", name, got[name], w)
		}
	}

	ops, err := p.GetOperations()
	if err != nil {
		t.Fatal(err)
	}

	for _, op := range ops["default"] {
		for _, param := range op.AllParams {
			if param.BaseName == "q" && (param.DataType != "string" || !param.IsNullable) {
				t.Errorf("q = %s nullable=%t, want a nullable string", param.DataType, param.IsNullable)
			}

			if param.BaseName == "file" && !param.IsFile {
				t.Errorf("file = %s, want a file", param.DataType)
			}
		}
	}
}
