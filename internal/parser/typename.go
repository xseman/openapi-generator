package parser

import (
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// isCompositeType reports whether the TypeScript type is a union or intersection
// built by applyCompositeType. Such values have no single (de)serializer helper,
// so callers must pass them through instead of appending FromJSON/ToJSON.
func isCompositeType(dataType string) bool {
	return strings.Contains(dataType, " | ") || strings.Contains(dataType, " & ")
}

func isPrimitiveType(t string) bool {
	primitives := map[string]bool{
		"string": true, "number": true, "boolean": true,
		"any": true, "void": true, "null": true,
		"Date": true, "Blob": true,
	}
	return primitives[t]
}

func (p *Parser) getSchemaType(schemaType, format string) string {
	if p.GetTypeFunc != nil {
		return p.GetTypeFunc(schemaType, format)
	}

	// Default TypeScript mappings
	switch schemaType {
	case "integer", "number":
		return "number"
	case "boolean":
		return "boolean"
	case "string":
		switch format {
		case "date", "date-time":
			return "Date"
		case "binary":
			return "Blob"
		default:
			return "string"
		}
	case "array":
		return "Array"
	case "object":
		return "any"
	default:
		if schemaType == "" {
			return "any"
		}
		return schemaType
	}
}

func (p *Parser) getTypeDeclaration(schema *openapi3.Schema) string {
	if schema == nil {
		return "any"
	}

	// Handle allOf - return the first ref if all are refs
	if len(schema.AllOf) > 0 {
		// Collect all non-nil refs
		refs := make([]string, 0, len(schema.AllOf))
		for _, ref := range schema.AllOf {
			if ref.Ref != "" {
				refName := extractRefName(ref.Ref)
				modelName := p.toModelName(refName)
				refs = append(refs, modelName)
			}
		}
		// If we have refs, return the first one (for oneOf member naming)
		// The actual intersection will be handled by the model generator
		if len(refs) > 0 {
			return refs[0]
		}
	}

	schemaType := ""
	if schema.Type != nil && len(schema.Type.Slice()) > 0 {
		schemaType = schema.Type.Slice()[0]
	}

	return p.getSchemaType(schemaType, schema.Format)
}

func (p *Parser) toModelName(name string) string {
	if p.ToModelNameFunc != nil {
		return p.ToModelNameFunc(name)
	}
	return toPascalCase(name)
}

func (p *Parser) toVarName(name string) string {
	if p.ToVarNameFunc != nil {
		return p.ToVarNameFunc(name)
	}
	return toCamelCase(name)
}

// toEnumName builds a PascalCase enum type name from a property or parameter name,
// suffixed with "Enum". toPascalCase splits on both dots and camelCase boundaries, so
// dot-namespaced names (e.g. "status.equals", "type.in") yield properly cased segments
// rather than gluing the suffix in lowercase. The resulting name is always further
// prefixed with the operation or model name by the generator, so it never stands alone
// as a reserved word — keeping this free of the reserved-word guarding that toModelName
// applies.
func (p *Parser) toEnumName(name string) string {
	return toPascalCase(name) + "Enum"
}
