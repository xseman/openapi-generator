package parser

import (
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

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

// discriminatorMapping flattens a discriminator mapping to the raw $ref each
// value names, the shape CodegenDiscriminator.Mapping carries.
func discriminatorMapping(mapping map[string]openapi3.MappingRef) map[string]string {
	if mapping == nil {
		return nil
	}

	out := make(map[string]string, len(mapping))
	for name, ref := range mapping {
		out[name] = ref.Ref
	}

	return out
}

// bound renders one side of a numeric range for the templates: the limit and
// whether it is exclusive. OAS 3.0 marks limit exclusive with a boolean; OAS 3.1
// writes the exclusive bound as its own number, which then stands in for limit.
func bound(limit *float64, exclusive openapi3.ExclusiveBound) (string, bool) {
	if exclusive.Value != nil {
		return fmt.Sprintf("%v", *exclusive.Value), true
	}

	if limit == nil {
		return "", exclusive.IsTrue()
	}

	return fmt.Sprintf("%v", *limit), exclusive.IsTrue()
}
