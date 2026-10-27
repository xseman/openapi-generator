package parser

import "testing"

// TestBasePathFillsServerVariables checks that the base path is the first
// server's URL with each variable's default in place.
func TestBasePathFillsServerVariables(t *testing.T) {
	spec := []byte(`
openapi: 3.0.0
info: {title: Servers, version: 1.0.0}
servers:
  - url: "{scheme}://api.example.com/{version}"
    variables:
      scheme: {default: https, enum: [https, http]}
      version: {default: v2}
paths: {}
`)

	p := NewParser()

	p.SkipValidation = true
	if err := p.LoadFromData(spec); err != nil {
		t.Fatalf("LoadFromData: %v", err)
	}

	if got, want := p.GetBasePath(), "https://api.example.com/v2"; got != want {
		t.Errorf("GetBasePath() = %q, want %q", got, want)
	}
}
