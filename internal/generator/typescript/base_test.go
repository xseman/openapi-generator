package typescript

import "testing"

// TestToModelName covers the model-name derivation bugs fixed in
// toTypescriptTypeName: dotted/namespaced Kubernetes-style schema names must
// camelize per-segment instead of collapsing into one run, acronym runs must
// survive camelization untouched, and names colliding with TypeScript
// primitives/utility types must be renamed via the "Model" safe-prefix.
func TestToModelName(t *testing.T) {
	g := NewBaseGenerator()

	tests := []struct {
		name string
		want string
	}{
		// Dotted/namespaced schema names (the highest-impact bug: dots must
		// become a word boundary, not be deleted, so each segment is
		// capitalized independently).
		{
			"io.k8s.api.admissionregistration.v1.NamedRuleWithOperations",
			"IoK8sApiAdmissionregistrationV1NamedRuleWithOperations",
		},
		{
			// Also exercises a hyphen inside a dotted segment.
			"io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.JSON",
			"IoK8sApiextensionsApiserverPkgApisApiextensionsV1JSON",
		},

		// Acronym runs must not be mangled into e.g. "Json" or "Httpgetaction".
		{"JSON", "JSON"},
		{"HTTPGetAction", "HTTPGetAction"},
		{"IPBlock", "IPBlock"},
		{"APIResource", "APIResource"},
		{"CSIDriver", "CSIDriver"},
		{"OAuth2", "OAuth2"},
		{"IPv4", "IPv4"},
		{"x509", "X509"},
		{"v1beta1", "V1beta1"},

		// TypeScript builtin/utility-type collisions get a "Model" prefix,
		// mirroring the pre-existing Error -> ModelError mechanism.
		{"Error", "ModelError"},
		{"Record", "ModelRecord"},
		{"Partial", "ModelPartial"},
		{"object", "ModelObject"},
		{"Pick", "ModelPick"},

		// A plain name is untouched.
		{"Pet", "Pet"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := g.ToModelName(tt.name); got != tt.want {
				t.Errorf("ToModelName(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

// TestToVarName covers ToVarName's property-identifier derivation, including
// the two defects a critic found in round 2: "$" must survive (Java's
// toVarName uses removeCharRegEx "[^\\w$]", not the default "\\W"), and a name
// that sanitizes down to a bare "_" must not collapse to the empty string
// (Java rewrites it to "_u" before camelizing).
func TestToVarName(t *testing.T) {
	g := NewBaseGenerator()

	tests := []struct {
		name string
		want string
	}{
		// "$" must be preserved, not stripped, e.g. JSON Schema's "$ref"/"$schema".
		{"$ref", "$ref"},
		{"$schema", "$schema"},

		// A name that sanitizes to a bare "_" must not camelize to "" (which
		// would emit `?: string;` with no identifier — a TS1131 syntax error).
		{".", "u"},
		{"-", "u"},
		{"_", "u"},

		// Leading digit gets a "_" escape, keeping the wire key (handled by the
		// caller, not tested here) as the unescaped original.
		{"123code", "_123code"},

		// Reserved TypeScript keywords get a "_" escape.
		{"class", "_class"},
		{"export", "_export"},

		// Built-in type names remain valid identifiers unescaped (only true
		// keywords are escaped).
		{"file", "file"},

		// Unmapped punctuation is dropped, not turned into a separator.
		{"a+b", "ab"},

		// Separators (dot, hyphen, space, colon) become camelCase boundaries.
		{"first-name", "firstName"},
		{"last.name", "lastName"},
		{"has space", "hasSpace"},

		// Leading/trailing underscores are stripped by camelization (matching
		// Java, not preserved as a distinct name).
		{"_leadingUnderscore", "leadingUnderscore"},
		{"trailingUnderscore_", "trailingUnderscore"},

		// A leading "@" is translated to "at_" so "@type" and a sibling "type"
		// property don't collide.
		{"@type", "atType"},
		{"@context", "atContext"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := g.ToVarName(tt.name); got != tt.want {
				t.Errorf("ToVarName(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

// TestToParamName mirrors the ToVarName cases most relevant to parameters:
// the "$" preservation and bare-"_" guard both apply there too (Java's
// toParamName uses the same sanitizeName(name, "[^\\w$]") call as toVarName),
// plus the reserved-keyword escaping that toSafeIdentifier adds for both.
func TestToParamName(t *testing.T) {
	g := NewBaseGenerator()

	tests := []struct {
		name string
		want string
	}{
		{"$ref", "$ref"},
		{".", "u"},
		{"123code", "_123code"},
		{"class", "_class"},
		{"user-id", "userId"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := g.ToParamName(tt.name); got != tt.want {
				t.Errorf("ToParamName(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

// TestSanitizeName covers the type/API-name/operation-ID sanitization path,
// which (unlike ToVarName/ToParamName) drops "$" rather than preserving it,
// matching Java's default sanitizeName(name, "\\W").
func TestSanitizeName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"+1", "plus1"},
		{"-1", "minus1"},
		{"$", "value"},
		{"$ref", "ref"},
		{"input.name", "input_name"},
		{"input-name", "input_name"},
		{"a+b", "ab"},
		{"input[a][b]", "input_a_b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeName(tt.name); got != tt.want {
				t.Errorf("SanitizeName(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

// TestCamelize exercises the acronym-preservation behavior Camelize relies on
// (splitting before every uppercase letter happens to leave all-caps runs
// intact) plus the "$" carve-out added for sanitizeIdentifierName's "$ref"
// case.
func TestCamelize(t *testing.T) {
	tests := []struct {
		name           string
		lowercaseFirst bool
		want           string
	}{
		{"JSONSchemaProps", false, "JSONSchemaProps"},
		{"HTTPGetAction", false, "HTTPGetAction"},
		{"io_k8s_api_v1_JSON", false, "IoK8sApiV1JSON"},
		{"$ref", true, "$ref"},
		{"$schema", true, "$schema"},
		{"_u", true, "u"},
		{"first_name", true, "firstName"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Camelize(tt.name, tt.lowercaseFirst); got != tt.want {
				t.Errorf("Camelize(%q, %v) = %q, want %q", tt.name, tt.lowercaseFirst, got, tt.want)
			}
		})
	}
}

// TestPrimitivesGuardsTSBuiltins checks that Primitives (consulted by
// toTypescriptTypeName to decide whether a camelized model name needs the
// "Model" safe-prefix) covers TypeScript's utility types in addition to the
// base language primitives, matching upstream's languageSpecificPrimitives.
func TestPrimitivesGuardsTSBuiltins(t *testing.T) {
	for _, name := range []string{
		"object", "Record", "Partial", "Required", "Readonly", "Pick", "Omit",
		"Exclude", "Extract", "NonNullable", "Parameters", "ReturnType",
		"InstanceType", "Uppercase", "Lowercase", "Capitalize", "Uncapitalize",
	} {
		if !Primitives[name] {
			t.Errorf("Primitives[%q] = false, want true", name)
		}
	}
}
