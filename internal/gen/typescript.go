package gen

import (
	"fmt"
	"strings"

	"github.com/xseman/openapi-generator/internal/config"
	"github.com/xseman/openapi-generator/internal/generator"
	"github.com/xseman/openapi-generator/internal/generator/typescript"
)

// applyTypeScriptAdditionalProperties binds the well-known typescript-fetch
// additional-property keys onto tsConfig, skipping any key that isn't
// present or doesn't match the expected type.
func applyTypeScriptAdditionalProperties(tsConfig *config.TypeScriptFetchConfig, props map[string]any) {
	if v, ok := props["withPackageJson"].(bool); ok {
		tsConfig.WithPackageJson = v
	}
	if v, ok := props["withInterfaces"].(bool); ok {
		tsConfig.WithInterfaces = v
	}
	if v, ok := props["useSingleRequestParameter"].(bool); ok {
		tsConfig.UseSingleRequestParameter = v
	}
	if v, ok := props["prefixParameterInterfaces"].(bool); ok {
		tsConfig.PrefixParameterInterfaces = v
	}
	if v, ok := props["withoutRuntimeChecks"].(bool); ok {
		tsConfig.WithoutRuntimeChecks = v
	}
	if v, ok := props["stringEnums"].(bool); ok {
		tsConfig.StringEnums = v
	}
	if v, ok := props["importFileExtension"].(string); ok {
		tsConfig.ImportFileExtension = v
	}
	if v, ok := props["fileNaming"].(string); ok {
		tsConfig.FileNaming = v
	}
	if v, ok := props["validationAttributes"].(bool); ok {
		tsConfig.GenerateValidationAttributes = v
	}
}

// toTsImports converts a list of model class names into the
// {classname, filename} maps expected by the TypeScript templates.
func toTsImports(imports []string, gen *typescript.FetchGenerator) []map[string]string {
	result := make([]map[string]string, 0, len(imports))
	for _, imp := range imports {
		className := gen.ToModelName(imp)
		// Skip empty class names and primitive types
		if className == "" || gen.IsPrimitive(className) {
			continue
		}
		result = append(result, map[string]string{
			"classname": className,
			"filename":  gen.ToModelFilename(className),
		})
	}
	return result
}

// isPrimitiveTypeTS checks if a type string is a TypeScript primitive type.
// Returns true for built-in types like string, number, boolean, etc.
func isPrimitiveTypeTS(t string) bool {
	primitives := map[string]bool{
		"string": true, "number": true, "boolean": true,
		"any": true, "void": true, "null": true,
		"Date": true, "Blob": true, "undefined": true,
	}
	return primitives[t]
}

// modelUsesRuntimeJSONHelper reports whether modelGeneric.mustache will
// literally emit a call to the runtime's {{datatype}}FromJSON/ToJSON helper
// named wantDatatype (e.g. "any" -> anyFromJSON/anyToJSON, "Blob" ->
// BlobFromJSON/BlobToJSON) for this model. That happens for a scalar var
// whose datatype resolved to wantDatatype outside of the
// isFreeFormObject/isPrimitiveType fast paths (which pass the value through
// directly instead), and for an array var whose item type resolved the same
// way (which maps the helper over the elements). Map vars never call it:
// modelGeneric.mustache only invokes a per-item converter for maps when the
// item is a model (mapValues({{items.datatype}}FromJSON)).
func modelUsesRuntimeJSONHelper(model *generator.CodegenModel, wantDatatype string) bool {
	callsHelper := func(p *generator.CodegenProperty) bool {
		return p != nil && p.Datatype == wantDatatype && !p.IsPrimitiveType && !p.IsFreeFormObject
	}
	for _, v := range model.Vars {
		if v == nil {
			continue
		}
		if !v.IsArray && !v.IsMap && callsHelper(v) {
			return true
		}
		if v.IsArray && v.Items != nil && !v.Items.IsContainer && callsHelper(v.Items) {
			return true
		}
	}
	return false
}

// generateModelIndex emits the per-folder barrel for the TypeScript models
// package.
func generateModelIndex(models []*generator.CodegenModel, gen *typescript.FetchGenerator) string {
	var sb strings.Builder
	sb.WriteString("/* tslint:disable */\n")
	sb.WriteString("/* eslint-disable */\n")
	sb.WriteString("\n")

	// Export helper functions from runtime
	sb.WriteString("// Re-export helper functions from runtime\n")
	sb.WriteString("export {\n")
	sb.WriteString("    anyFromJSON,\n")
	sb.WriteString("    anyToJSON,\n")
	sb.WriteString("    BlobFromJSON,\n")
	sb.WriteString("    BlobToJSON,\n")
	sb.WriteString("    FromJSON,\n")
	sb.WriteString("} from '../runtime';\n")
	sb.WriteString("\n")

	// Add ModelObject type for generic object schemas
	sb.WriteString("// Generic object type for unstructured schemas\n")
	sb.WriteString("export type ModelObject = Record<string, any>;\n")
	sb.WriteString("export function ModelObjectFromJSON(json: any): ModelObject {\n")
	sb.WriteString("    return json;\n")
	sb.WriteString("}\n")
	sb.WriteString("export function ModelObjectToJSON(value: ModelObject): any {\n")
	sb.WriteString("    return value;\n")
	sb.WriteString("}\n")
	sb.WriteString("\n")

	for _, model := range models {
		filename := gen.ToModelFilename(model.Classname)
		ext := gen.ImportFileExtension
		fmt.Fprintf(&sb, "export * from './%s%s';\n", filename, ext)
	}

	return sb.String()
}

// generateApiIndex emits the per-folder barrel for the TypeScript apis
// package.
func generateApiIndex(ops map[string][]*generator.CodegenOperation, gen *typescript.FetchGenerator) string {
	var sb strings.Builder
	sb.WriteString("/* tslint:disable */\n")
	sb.WriteString("/* eslint-disable */\n")

	for _, tag := range sortedTags(ops) {
		apiClassname := gen.ToApiName(tag)
		filename := gen.ToApiFilename(apiClassname)
		ext := gen.ImportFileExtension
		fmt.Fprintf(&sb, "export * from './%s%s';\n", filename, ext)
	}

	return sb.String()
}

// PrintTypeScriptFetchConfigHelp prints the typescript-fetch additional
// config options to stdout (used by the "config-help" CLI command).
func PrintTypeScriptFetchConfigHelp() {
	fmt.Println("CONFIG OPTIONS for typescript-fetch:")
	fmt.Println()
	fmt.Println("  withPackageJson")
	fmt.Println("      Generate package.json and tsconfig.json files. (Default: false)")
	fmt.Println()
	fmt.Println("  withInterfaces")
	fmt.Println("      Generate interfaces alongside classes. (Default: false)")
	fmt.Println()
	fmt.Println("  useSingleRequestParameter")
	fmt.Println("      Use single request object for method parameters. (Default: true)")
	fmt.Println()
	fmt.Println("  prefixParameterInterfaces")
	fmt.Println("      Prefix parameter interfaces with API class name. (Default: false)")
	fmt.Println()
	fmt.Println("  withoutRuntimeChecks")
	fmt.Println("      Skip runtime type validation (FromJSON/ToJSON). (Default: false)")
	fmt.Println()
	fmt.Println("  stringEnums")
	fmt.Println("      Generate string enums instead of const objects. (Default: false)")
	fmt.Println()
	fmt.Println("  importFileExtension")
	fmt.Println("      File extension for imports (e.g., '.js' for ESM). (Default: '')")
	fmt.Println()
	fmt.Println("  fileNaming")
	fmt.Println("      File naming convention: PascalCase, camelCase, kebab-case.")
	fmt.Println("      (Default: kebab-case)")
	fmt.Println()
	fmt.Println("  validationAttributes")
	fmt.Println("      Generate validation metadata. (Default: false)")
	fmt.Println()
}
