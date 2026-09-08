package gen

import (
	"sort"
	"strings"

	"github.com/xseman/openapi-generator/internal/generator"
)

// parseAdditionalProperties converts a list of "key=value" strings (as
// accepted via the CLI's --additional-properties flag) into a map, coercing
// "true"/"false" values to booleans so downstream config binding can type
// assert on bool.
func parseAdditionalProperties(props []string) map[string]any {
	result := make(map[string]any)
	for _, prop := range props {
		parts := strings.SplitN(prop, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			switch strings.ToLower(value) {
			case "true":
				result[key] = true
			case "false":
				result[key] = false
			default:
				result[key] = value
			}
		}
	}
	return result
}

// extractHost pulls the host (authority) component out of an absolute
// basePath URL, e.g. "https://api.example.com/v1" -> "api.example.com".
func extractHost(basePath string) string {
	if strings.HasPrefix(basePath, "http://") || strings.HasPrefix(basePath, "https://") {
		parts := strings.SplitN(basePath, "/", 4)
		if len(parts) >= 3 {
			return parts[2]
		}
	}
	return ""
}

// copyMap creates a shallow copy of a map.
func copyMap(m map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range m {
		result[k] = v
	}
	return result
}

// sortedTags returns the tag keys of opsByTag in deterministic (alphabetical)
// order. Go map iteration is randomised, so ranging over the map directly
// would reorder generated API files and barrel exports on every run;
// callers that emit output must iterate via this helper instead.
func sortedTags(opsByTag map[string][]*generator.CodegenOperation) []string {
	tags := make([]string, 0, len(opsByTag))
	for tag := range opsByTag {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}

// collectApiImports gathers the deduplicated, sorted set of model imports
// referenced by a group of operations, resolving each to its class and file
// name and filtering out primitives.
func collectApiImports(ops []*generator.CodegenOperation, gen generator.CodegenConfig) []map[string]string {
	imports := make(map[string]bool)
	for _, op := range ops {
		for _, imp := range op.Imports {
			imports[imp] = true
		}
	}

	// Get primitives map to filter
	primitives := gen.GetLanguageSpecificPrimitives()

	result := make([]map[string]string, 0, len(imports))
	for imp := range imports {
		// Convert the import name to the proper model class name
		className := gen.ToModelName(imp)
		// Skip empty class names and primitive types
		if className == "" || primitives[className] {
			continue
		}
		result = append(result, map[string]string{
			"import":    imp,
			"classname": className,
			"className": className, // Template expects className (camelCase)
			"filename":  gen.ToModelFilename(className),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i]["classname"] < result[j]["classname"]
	})

	return result
}
