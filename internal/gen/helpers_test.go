package gen

import (
	"reflect"
	"testing"

	"github.com/xseman/openapi-generator/internal/generator"
)

func TestParseAdditionalProperties(t *testing.T) {
	tests := []struct {
		name  string
		props []string
		want  map[string]any
	}{
		{
			name:  "nil input",
			props: nil,
			want:  map[string]any{},
		},
		{
			name:  "string value",
			props: []string{"fileNaming=kebab-case"},
			want:  map[string]any{"fileNaming": "kebab-case"},
		},
		{
			name:  "boolean values are coerced case-insensitively",
			props: []string{"withPackageJson=true", "withInterfaces=FALSE"},
			want: map[string]any{
				"withPackageJson": true,
				"withInterfaces":  false,
			},
		},
		{
			name:  "whitespace around key and value is trimmed",
			props: []string{" pubName = my_pkg "},
			want:  map[string]any{"pubName": "my_pkg"},
		},
		{
			name:  "value may itself contain an equals sign",
			props: []string{"query=a=b"},
			want:  map[string]any{"query": "a=b"},
		},
		{
			name:  "entries without '=' are silently ignored",
			props: []string{"noEquals", "valid=1"},
			want:  map[string]any{"valid": "1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAdditionalProperties(tt.props)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseAdditionalProperties(%v) = %#v, want %#v", tt.props, got, tt.want)
			}
		})
	}
}

func TestExtractHost(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		want     string
	}{
		{"https URL with path", "https://api.example.com/v1", "api.example.com"},
		{"http URL with path", "http://localhost:8080/api", "localhost:8080"},
		{"https URL without path", "https://api.example.com", "api.example.com"},
		{"relative path", "/v1", ""},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractHost(tt.basePath); got != tt.want {
				t.Errorf("extractHost(%q) = %q, want %q", tt.basePath, got, tt.want)
			}
		})
	}
}

func TestIsPrimitiveTypeTS(t *testing.T) {
	tests := []struct {
		typ  string
		want bool
	}{
		{"string", true},
		{"number", true},
		{"boolean", true},
		{"any", true},
		{"void", true},
		{"null", true},
		{"Date", true},
		{"Blob", true},
		{"undefined", true},
		{"Array<string>", false},
		{"MyModel", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.typ, func(t *testing.T) {
			if got := isPrimitiveTypeTS(tt.typ); got != tt.want {
				t.Errorf("isPrimitiveTypeTS(%q) = %v, want %v", tt.typ, got, tt.want)
			}
		})
	}
}

func TestSortedTags(t *testing.T) {
	tests := []struct {
		name string
		in   map[string][]*generator.CodegenOperation
		want []string
	}{
		{
			name: "nil map",
			in:   nil,
			want: []string{},
		},
		{
			name: "unordered keys are sorted alphabetically",
			in: map[string][]*generator.CodegenOperation{
				"zebra": nil,
				"alpha": nil,
				"mid":   nil,
			},
			want: []string{"alpha", "mid", "zebra"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortedTags(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("sortedTags(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestCopyMap(t *testing.T) {
	original := map[string]any{"a": 1, "b": "two"}
	copied := copyMap(original)

	if !reflect.DeepEqual(original, copied) {
		t.Fatalf("copyMap(%v) = %v, want equal contents", original, copied)
	}

	// Mutating the copy must not affect the original.
	copied["a"] = 999
	copied["c"] = "new"
	if original["a"] != 1 {
		t.Errorf("original map was mutated via the copy: original[\"a\"] = %v, want 1", original["a"])
	}
	if _, exists := original["c"]; exists {
		t.Errorf("original map gained a key added to the copy")
	}

	// copyMap of nil should return an empty, non-nil map.
	empty := copyMap(nil)
	if empty == nil || len(empty) != 0 {
		t.Errorf("copyMap(nil) = %v, want empty non-nil map", empty)
	}
}
