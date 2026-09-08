package parser

import (
	"sort"

	"github.com/xseman/openapi-generator/internal/codegen"
)

// collectPropertyImports records every model type referenced by prop into imports,
// including types nested arbitrarily deep inside container properties. Both array
// element types and map (additionalProperties) value types are carried on
// prop.Items, and either can itself be another container — e.g. Array<Array<Cell>>
// or Map<string, Array<Cell>> — so Items is walked recursively rather than just one
// level, which previously left the innermost model type (e.g. Cell) unimported.
// selfClassname is excluded to avoid circular self-imports; pass "" when there is no
// enclosing model (e.g. collecting imports for an operation parameter).
func collectPropertyImports(prop *codegen.CodegenProperty, selfClassname string, imports map[string]bool) {
	if prop == nil {
		return
	}
	if prop.IsModel && prop.DataType != selfClassname && !isPrimitiveType(prop.DataType) {
		imports[prop.DataType] = true
	}
	for _, m := range prop.ComposedModels {
		if m != selfClassname && !isPrimitiveType(m) {
			imports[m] = true
		}
	}
	collectPropertyImports(prop.Items, selfClassname, imports)
}

func (p *Parser) collectImports(model *codegen.CodegenModel) []string {
	imports := make(map[string]bool)

	for _, prop := range model.Vars {
		collectPropertyImports(prop, model.Classname, imports)
	}
	// A bare-map model (type: object + additionalProperties, no named properties
	// of its own) has no model.Vars at all — its only referenced type lives on
	// AdditionalProperties, which is otherwise never visited above.
	collectPropertyImports(model.AdditionalProperties, model.Classname, imports)

	for _, ref := range model.OneOf {
		// Skip self-references to avoid circular imports
		if ref != model.Classname && !isPrimitiveType(ref) {
			imports[ref] = true
		}
	}
	for _, ref := range model.AnyOf {
		// Skip self-references to avoid circular imports
		if ref != model.Classname && !isPrimitiveType(ref) {
			imports[ref] = true
		}
	}
	for _, ref := range model.AllOf {
		if ref != model.Classname && !isPrimitiveType(ref) {
			imports[ref] = true
		}
	}

	result := make([]string, 0, len(imports))
	for imp := range imports {
		result = append(result, imp)
	}
	sort.Strings(result)
	return result
}

func (p *Parser) collectOperationImports(op *codegen.CodegenOperation) []string {
	imports := make(map[string]bool)

	// From parameters
	for _, param := range op.AllParams {
		if param.IsModel && !isPrimitiveType(param.DataType) && !isCompositeType(param.DataType) {
			imports[param.DataType] = true
		}
		// Member models of a union/intersection parameter type.
		for _, m := range param.ComposedModels {
			imports[m] = true
		}
		// Nested container element types (arrays of arrays, maps of arrays, etc.)
		// are carried on param.Items regardless of nesting depth.
		if param.Items != nil {
			collectPropertyImports(param.Items, "", imports)
		}
	}

	// From return type - also check that ReturnBaseType is not primitive. A
	// composite (union/intersection) return type imports its member models
	// instead of the joined string, which is not an importable identifier;
	// such returns carry ReturnTypeIsPrimitive so no FromJSON import is needed.
	if op.ReturnType != "" && !op.ReturnTypeIsPrimitive &&
		!isPrimitiveType(op.ReturnType) && !isPrimitiveType(op.ReturnBaseType) && !isCompositeType(op.ReturnBaseType) {
		imports[op.ReturnBaseType] = true
	}
	for _, m := range op.ReturnComposedModels {
		imports[m] = true
	}

	result := make([]string, 0, len(imports))
	for imp := range imports {
		result = append(result, imp)
	}
	sort.Strings(result)
	return result
}
