package parser

import (
	"fmt"
	"sort"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/xseman/openapi-generator/internal/codegen"
)

// GetModels extracts all models from the OpenAPI spec.
func (p *Parser) GetModels() ([]*codegen.CodegenModel, error) {
	if p.Doc == nil || p.Doc.Components == nil {
		return nil, nil
	}

	var models []*codegen.CodegenModel

	// Get schema names in sorted order for deterministic output
	schemaNames := make([]string, 0, len(p.Doc.Components.Schemas))
	for name := range p.Doc.Components.Schemas {
		schemaNames = append(schemaNames, name)
	}

	sort.Strings(schemaNames)

	for _, name := range schemaNames {
		schemaRef := p.Doc.Components.Schemas[name]
		if schemaRef == nil || schemaRef.Value == nil {
			continue
		}

		model := p.schemaToModel(name, schemaRef.Value)
		models = append(models, model)
	}

	// Mark models whose parent is a oneOf union: TypeScript does not allow an
	// interface to extend a union type, so templates emit an intersection type
	// alias for these models instead.
	oneOfClassnames := make(map[string]bool)

	for _, m := range models {
		if len(m.OneOf) > 0 {
			oneOfClassnames[m.Classname] = true
		}
	}

	for _, m := range models {
		if m.Parent != "" && oneOfClassnames[m.Parent] {
			m.ParentIsOneOf = true
		}
	}

	return models, nil
}

// schemaToModel converts an OpenAPI schema to a CodegenModel.
func (p *Parser) schemaToModel(name string, schema *openapi3.Schema) *codegen.CodegenModel {
	model := &codegen.CodegenModel{
		Name:                 name,
		SchemaName:           name,
		Classname:            p.toModelName(name),
		ClassVarName:         p.toVarName(name),
		ClassFilename:        p.toModelName(name),
		Title:                schema.Title,
		Description:          escapeUnsafeChars(schema.Description),
		UnescapedDescription: escapeUnsafeChars(schema.Description),
		IsNullable:           schema.Nullable,
		IsDeprecated:         schema.Deprecated,
		VendorExtensions:     convertExtensions(schema.Extensions),
	}

	// Determine schema type
	schemaType := schema.Type
	if schemaType == nil || len(schemaType.Slice()) == 0 {
		// Try to infer type
		if len(schema.Enum) > 0 {
			model.IsEnum = true
		} else if schema.Properties != nil {
			schemaType = &openapi3.Types{"object"}
		}
	}

	// Handle enum
	if len(schema.Enum) > 0 {
		model.IsEnum = true
		model.AllowableValues = map[string]any{
			"values": schema.Enum,
		}
	}

	// Handle type-specific logic
	if schemaType != nil && len(schemaType.Slice()) > 0 {
		primaryType := schemaType.Slice()[0]

		switch primaryType {
		case "object":
			model.HasVars = len(schema.Properties) > 0
			model.Vars = p.extractProperties(schema, model)
			model.AllVars = model.Vars
			model.RequiredVars = filterRequired(model.Vars)
			model.OptionalVars = filterOptional(model.Vars)
			model.ReadOnlyVars = filterReadOnly(model.Vars)
			model.HasRequired = len(model.RequiredVars) > 0
			model.HasOptional = len(model.OptionalVars) > 0
			model.HasReadOnly = len(model.ReadOnlyVars) > 0

			// Additional properties
			if schema.AdditionalProperties.Has != nil && *schema.AdditionalProperties.Has {
				model.IsAdditionalPropertiesTrue = true
			}

			if schema.AdditionalProperties.Schema != nil {
				// Resolve the full property shape, not just its data type, and keep
				// it on the model: container values render with their parameters
				// (e.g. Array<Rule>, not a bare "Array"), and collectImports below
				// needs the full CodegenProperty (with its own possibly-nested
				// Items) to find every model referenced by a bare-map model — one
				// with no named properties of its own, only additionalProperties —
				// since such a model has no model.Vars for collectImports to walk.
				model.AdditionalProperties = p.schemaRefToProperty("value", schema.AdditionalProperties.Schema, false)
				model.AdditionalPropertiesType = model.AdditionalProperties.DataType
			}

		case "array":
			model.IsArray = true
			if schema.Items != nil && schema.Items.Value != nil {
				model.Items = p.schemaToProperty("items", schema.Items.Value, false)
				model.ArrayModelType = model.Items.DataType
			}

		case "string":
			model.IsString = true
			model.IsPrimitiveType = true

		case "integer":
			model.IsInteger = true
			model.IsNumeric = true
			model.IsPrimitiveType = true

		case "number":
			model.IsNumber = true
			model.IsNumeric = true
			model.IsPrimitiveType = true

		case "boolean":
			model.IsBoolean = true
			model.IsPrimitiveType = true
		}

		model.DataType = p.getSchemaType(primaryType, schema.Format)
	}

	if len(schema.OneOf) > 0 {
		p.oneOfMembers(model, schema)
	}

	if len(schema.AnyOf) > 0 {
		model.AnyOf = make([]string, 0, len(schema.AnyOf))
		for _, ref := range schema.AnyOf {
			if ref.Ref != "" {
				model.AnyOf = append(model.AnyOf, extractRefName(ref.Ref))
			}
		}
	}

	if len(schema.AllOf) > 0 {
		model.AllOf = make([]string, 0, len(schema.AllOf))
		for _, ref := range schema.AllOf {
			if ref.Ref != "" {
				refName := extractRefName(ref.Ref)

				model.AllOf = append(model.AllOf, refName)
				if model.Parent == "" {
					// Convert to valid model name for TypeScript/other languages
					model.Parent = p.toModelName(refName)
				}
			} else if ref.Value != nil && ref.Value.Properties != nil {
				// Inline properties from allOf
				props := p.extractProperties(ref.Value, model)
				model.Vars = append(model.Vars, props...)
			}
		}
	}

	// Handle discriminator
	if schema.Discriminator != nil {
		model.Discriminator = &codegen.CodegenDiscriminator{
			PropertyName:     p.toVarName(schema.Discriminator.PropertyName),
			PropertyBaseName: schema.Discriminator.PropertyName,
			Mapping:          schema.Discriminator.Mapping,
		}
		if len(schema.Discriminator.Mapping) > 0 {
			model.HasDiscriminatorWithNonEmptyMapping = true
			model.Discriminator.MappedModels = make([]*codegen.MappedModel, 0)
			// Iterate mapping keys in sorted order for stable output.
			mappingNames := make([]string, 0, len(schema.Discriminator.Mapping))
			for mappingName := range schema.Discriminator.Mapping {
				mappingNames = append(mappingNames, mappingName)
			}

			sort.Strings(mappingNames)

			for _, mappingName := range mappingNames {
				mapped := &codegen.MappedModel{
					MappingName: mappingName,
					ModelName:   extractRefName(schema.Discriminator.Mapping[mappingName]),
				}
				// A mapping that resolves to this model itself would make the
				// generated FromJSONTyped call itself forever; divert it to the
				// self-referencing slot the template guards with ignoreDiscriminator.
				if p.toModelName(mapped.ModelName) == model.Classname {
					model.SelfReferencingDiscriminatorMapping = mapped
					model.HasSelfReferencingDiscriminatorMapping = true

					continue
				}

				model.Discriminator.MappedModels = append(model.Discriminator.MappedModels, mapped)
			}
		}
	}

	// Collect imports
	model.Imports = p.collectImports(model)

	// Set validation properties
	model.Pattern = schema.Pattern
	if schema.Min != nil {
		model.Minimum = fmt.Sprintf("%v", *schema.Min)
	}

	if schema.Max != nil {
		model.Maximum = fmt.Sprintf("%v", *schema.Max)
	}

	model.MinLength = intPtr(int(schema.MinLength))
	model.MaxLength = uint64ToIntPtr(schema.MaxLength)
	model.MinItems = intPtr(int(schema.MinItems))
	model.MaxItems = uint64ToIntPtr(schema.MaxItems)
	model.UniqueItems = schema.UniqueItems
	model.ExclusiveMinimum = schema.ExclusiveMin
	model.ExclusiveMaximum = schema.ExclusiveMax

	return model
}

func filterRequired(props []*codegen.CodegenProperty) []*codegen.CodegenProperty {
	var result []*codegen.CodegenProperty

	for _, p := range props {
		if p.Required {
			result = append(result, p)
		}
	}

	return result
}

func filterOptional(props []*codegen.CodegenProperty) []*codegen.CodegenProperty {
	var result []*codegen.CodegenProperty

	for _, p := range props {
		if !p.Required {
			result = append(result, p)
		}
	}

	return result
}

func filterReadOnly(props []*codegen.CodegenProperty) []*codegen.CodegenProperty {
	var result []*codegen.CodegenProperty

	for _, p := range props {
		if p.IsReadOnly {
			result = append(result, p)
		}
	}

	return result
}

// oneOfMembers fills model.OneOf and the import lists it needs: referenced
// models by name, inline members by their resolved type, with arrays of
// models and primitive members told apart for the template's conversions.
func (p *Parser) oneOfMembers(model *codegen.CodegenModel, schema *openapi3.Schema) {
	model.OneOf = make([]string, 0, len(schema.OneOf))
	models := make(map[string]bool) // deduplicated, sorted below
	arrays := make(map[string]bool)

	for _, ref := range schema.OneOf {
		if ref.Ref != "" {
			modelName := p.toModelName(extractRefName(ref.Ref))
			model.OneOf = append(model.OneOf, modelName)
			// Non-primitive models are imported.
			if !isPrimitiveType(modelName) {
				models[modelName] = true
			}

			continue
		}

		if ref.Value == nil {
			continue
		}
		// Members render as required: a null payload is handled before the
		// per-member conversion branches ever run.
		prop := p.schemaToProperty("member", ref.Value, true)
		// Prefer the resolved property type: it carries type parameters
		// (Array<Pet>) where getTypeDeclaration returns a bare "Array".
		// Composite declarations ("A & B") collapse to their first $ref so the
		// alias members stay assignable from the narrowing conversions below.
		typeName := prop.DataType
		if typeName == "" || typeName == "any" || isCompositeType(typeName) {
			typeName = p.getTypeDeclaration(ref.Value)
		}

		model.OneOf = append(model.OneOf, typeName)

		switch {
		case prop.IsArray && prop.ComplexType != "":
			// Array of models: import the item type so the template can narrow
			// with instanceOf on each element.
			arrays[prop.ComplexType] = true
		case prop.IsArray || prop.IsMap || (prop.IsPrimitiveType && !prop.IsFreeFormObject):
			// Primitive members (arrays of primitives included) get typed JSON
			// conversion branches instead of model imports.
			model.OneOfPrimitives = append(model.OneOfPrimitives, prop)
		case !isPrimitiveType(typeName) && !isCompositeType(typeName):
			// Model members, inline compositions that collapse to a single
			// referenced model included; free-form "any" is excluded by the guard.
			models[typeName] = true
		}
	}

	model.OneOfModels = sortedKeys(models)
	model.OneOfArrays = sortedKeys(arrays)
	model.HasOneOf = len(model.OneOf) > 0
}

// sortedKeys is a map's keys in order, an empty (not nil) slice for none, so
// the template sees [] rather than null.
func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}
