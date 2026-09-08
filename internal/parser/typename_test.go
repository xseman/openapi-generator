package parser

import "testing"

func TestIsPrimitiveType(t *testing.T) {
	tests := []struct {
		in   string
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
		{"Pet", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := isPrimitiveType(tt.in); got != tt.want {
				t.Errorf("isPrimitiveType(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsCompositeType(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"Foo | Bar", true},
		{"Foo & Bar", true},
		{"Array<Foo | Bar>", true},
		{"Foo", false},
		{"Array<Foo>", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := isCompositeType(tt.in); got != tt.want {
				t.Errorf("isCompositeType(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
