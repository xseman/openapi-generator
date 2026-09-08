package parser

import "strings"

// extractRefName returns the final path segment of an OpenAPI $ref, e.g.
// "#/components/schemas/Pet" -> "Pet".
func extractRefName(ref string) string {
	parts := strings.Split(ref, "/")
	return parts[len(parts)-1]
}

// convertExtensions normalizes a schema/operation/etc.'s vendor extensions map for
// template use, substituting an empty map for a nil one.
func convertExtensions(ext map[string]any) map[string]any {
	if ext == nil {
		return make(map[string]any)
	}
	return ext
}

// intPtr returns a pointer to i, or nil when i is the zero value. Used for
// validation fields (MinLength, MinItems, ...) that templates render only when set.
func intPtr(i int) *int {
	if i == 0 {
		return nil
	}
	return &i
}

// uint64ToIntPtr converts an optional uint64 validation bound (as kin-openapi
// represents MaxLength/MaxItems) to the *int the codegen model fields use.
func uint64ToIntPtr(v *uint64) *int {
	if v == nil {
		return nil
	}
	i := int(*v)
	return &i
}
