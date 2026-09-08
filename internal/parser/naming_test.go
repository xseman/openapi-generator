package parser

import (
	"reflect"
	"testing"
)

func TestSplitWords(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"camelCase", "fooBar", []string{"foo", "Bar"}},
		{"PascalCase", "FooBar", []string{"Foo", "Bar"}},
		{"snake_case", "foo_bar", []string{"foo", "bar"}},
		{"kebab-case", "foo-bar", []string{"foo", "bar"}},
		{"dot.namespaced", "status.equals", []string{"status", "equals"}},
		{"single word", "already", []string{"already"}},
		{"empty", "", nil},
		{"alnum boundary", "a1B2c3", []string{"a1", "B2c3"}},
		{"leading separators dropped", "__leading", []string{"leading"}},
		{"trailing separators dropped", "trailing__", []string{"trailing"}},
		// Consecutive uppercase letters each become their own word: the
		// boundary check fires on every uppercase rune once current is
		// non-empty, regardless of the previous rune's case.
		{"consecutive caps fragment", "ABCfoo", []string{"A", "B", "Cfoo"}},
		{"screaming snake fragments letter by letter", "SCREAMING_SNAKE_CASE", []string{
			"S", "C", "R", "E", "A", "M", "I", "N", "G",
			"S", "N", "A", "K", "E",
			"C", "A", "S", "E",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitWords(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitWords(%q) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}

func TestToCamelCase(t *testing.T) {
	tests := []struct{ in, want string }{
		{"fooBar", "fooBar"},
		{"FooBar", "fooBar"},
		{"foo_bar", "fooBar"},
		{"foo-bar", "fooBar"},
		{"status.equals", "statusEquals"},
		{"already", "already"},
		{"id", "id"},
		{"pet_id", "petId"},
		{"PetId", "petId"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := toCamelCase(tt.in); got != tt.want {
				t.Errorf("toCamelCase(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestToPascalCase(t *testing.T) {
	tests := []struct{ in, want string }{
		{"fooBar", "FooBar"},
		{"FooBar", "FooBar"},
		{"foo_bar", "FooBar"},
		{"foo-bar", "FooBar"},
		{"status.equals", "StatusEquals"},
		{"already", "Already"},
		{"id", "Id"},
		{"pet_id", "PetId"},
		{"PetId", "PetId"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := toPascalCase(tt.in); got != tt.want {
				t.Errorf("toPascalCase(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct{ in, want string }{
		{"fooBar", "foo_bar"},
		{"FooBar", "foo_bar"},
		{"foo_bar", "foo_bar"},
		{"foo-bar", "foo_bar"},
		{"status.equals", "status_equals"},
		{"already", "already"},
		{"id", "id"},
		{"pet_id", "pet_id"},
		{"PetId", "pet_id"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := toSnakeCase(tt.in); got != tt.want {
				t.Errorf("toSnakeCase(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestEscapeUnsafeChars(t *testing.T) {
	tests := []struct{ in, want string }{
		{"normal text", "normal text"},
		{"a */ b", "a *_/ b"},
		{"a /* b", "a /_* b"},
		{"*/ then /*", "*_/ then /_*"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := escapeUnsafeChars(tt.in); got != tt.want {
				t.Errorf("escapeUnsafeChars(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSanitizeTag(t *testing.T) {
	tests := []struct{ in, want string }{
		{"/pets/{id}", "PetsId"},
		{"/pets/{petId}/owner", "PetsPetIdOwner"},
		{"/a-b/{c}", "ABC"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := sanitizeTag(tt.in); got != tt.want {
				t.Errorf("sanitizeTag(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestToEnumVarName(t *testing.T) {
	tests := []struct{ in, want string }{
		{"CALL_CENTRUM", "CallCentrum"},
		{"C+", "CPlus"},
		// SCREAMING_SNAKE_CASE-shaped values collapse to one word per
		// underscore-separated run rather than fragmenting into individual
		// letters, unlike splitWords/toPascalCase on the same input: the
		// whole value is lower-cased before the PascalCase pass.
		{"ODPOCTAR", "Odpoctar"},
		{"already", "Already"},
		{"123abc", "_123Abc"},
		{"", "Empty"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := toEnumVarName(tt.in); got != tt.want {
				t.Errorf("toEnumVarName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
