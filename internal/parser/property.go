package parser

import (
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/xseman/openapi-generator/internal/codegen"
)

// extractProperties extracts properties from an object schema.
func (p *Parser) extractProperties(schema *openapi3.Schema, model *codegen.CodegenModel) []*codegen.CodegenProperty {
	if schema.Properties == nil {
		return nil
	}

	requiredSet := make(map[string]bool)
	for _, r := range schema.Required {
		requiredSet[r] = true
	}

	var props []*codegen.CodegenProperty
	for _, name := range p.orderedPropertyNames(schema) {
		propRef := schema.Properties[name]
		if propRef == nil || propRef.Value == nil {
			continue
		}

		required := requiredSet[name]
		prop := p.schemaRefToProperty(name, propRef, required)
		props = append(props, prop)
	}

	return props
}

// schemaRefToProperty builds a CodegenProperty from a property's SchemaRef,
// resolving a direct $ref to a named object schema into that model's type.
// schemaToProperty only receives the dereferenced *openapi3.Schema, so it cannot
// see the $ref; without this a property like {"$ref": "#/.../Zamestnanec"} would
// collapse to `any` via the "object" branch instead of typing as `Zamestnanec`.
func (p *Parser) schemaRefToProperty(name string, ref *openapi3.SchemaRef, required bool) *codegen.CodegenProperty {
	if ref == nil || ref.Value == nil {
		prop := &codegen.CodegenProperty{
			Name:             p.toVarName(name),
			BaseName:         name,
			Required:         required,
			DataType:         "any",
			Datatype:         "any",
			BaseType:         "any",
			DatatypeWithEnum: "any",
			IsPrimitiveType:  true,
			IsAnyType:        true,
		}
		return prop
	}

	prop := p.schemaToProperty(name, ref.Value, required)

	// Only object-like targets need the override: arrays, enums and primitives are
	// already typed correctly by schemaToProperty, and clobbering them with the
	// model name would be wrong.
	if ref.Ref != "" && (isObjectSchema(ref.Value) || len(ref.Value.Properties) > 0) {
		modelName := p.toModelName(extractRefName(ref.Ref))
		if modelName != "" && !isPrimitiveType(modelName) {
			prop.DataType = modelName
			prop.Datatype = modelName
			prop.BaseType = modelName
			prop.ComplexType = modelName
			prop.DatatypeWithEnum = modelName
			prop.IsModel = true
			prop.IsPrimitiveType = false
			prop.IsFreeFormObject = false
			prop.IsAnyType = false
		}
	}

	return prop
}

// applyMemberType resolves a single composition member ($ref or inline schema)
// onto prop, copying its type-bearing fields. Returns false if the member could
// not be resolved to a concrete (non-any) type, leaving prop untouched.
func (p *Parser) applyMemberType(prop *codegen.CodegenProperty, ref *openapi3.SchemaRef) bool {
	member := p.schemaRefToProperty(prop.BaseName, ref, prop.Required)
	if member == nil || member.DataType == "" || member.DataType == "any" {
		return false
	}
	prop.DataType = member.DataType
	prop.BaseType = member.BaseType
	prop.ComplexType = member.ComplexType
	prop.DatatypeWithEnum = member.DatatypeWithEnum
	prop.IsModel = member.IsModel
	prop.IsArray = member.IsArray
	prop.IsMap = member.IsMap
	prop.IsContainer = member.IsContainer
	prop.IsEnum = member.IsEnum
	prop.IsPrimitiveType = member.IsPrimitiveType
	prop.IsFreeFormObject = member.IsFreeFormObject
	prop.Items = member.Items
	prop.AllowableValues = member.AllowableValues
	prop.EnumName = member.EnumName
	return true
}

// applyCompositeType builds a union ("|") or intersection ("&") type from the
// composition members and registers the member models for import. No single
// (de)serializer exists for a union/intersection, so the value is passed through.
func (p *Parser) applyCompositeType(prop *codegen.CodegenProperty, refs openapi3.SchemaRefs, sep string) bool {
	names, models := p.memberTypeNames(refs)
	if len(names) == 0 {
		return false
	}
	joined := strings.Join(names, sep)
	prop.DataType = joined
	prop.BaseType = joined
	prop.DatatypeWithEnum = joined
	prop.IsPrimitiveType = false
	prop.IsFreeFormObject = true
	prop.ComposedModels = models
	return true
}

// memberTypeNames resolves composition members to their TypeScript type strings,
// using the referenced model for $ref members and the recursively resolved type
// for inline members. Empty/any members are dropped and duplicates removed.
// The second return value lists the bare importable model names the members
// reference (e.g. Foo for an Array<Foo> member), for import generation.
func (p *Parser) memberTypeNames(refs openapi3.SchemaRefs) ([]string, []string) {
	names := make([]string, 0, len(refs))
	models := make([]string, 0, len(refs))
	seen := make(map[string]bool)
	seenModels := make(map[string]bool)
	for _, m := range refs {
		if m == nil {
			continue
		}
		var t, imp string
		switch {
		case m.Ref != "":
			t = p.toModelName(extractRefName(m.Ref))
			imp = t
		case m.Value != nil:
			mp := p.schemaToProperty("member", m.Value, false)
			t = mp.DataType
			switch {
			case mp.ComplexType != "":
				imp = mp.ComplexType
			case mp.IsModel:
				imp = t
			}
		}
		if t == "" || t == "any" || seen[t] {
			continue
		}
		seen[t] = true
		names = append(names, t)
		if imp != "" && !isPrimitiveType(imp) && !seenModels[imp] {
			seenModels[imp] = true
			models = append(models, imp)
		}
	}
	return names, models
}

// schemaToProperty converts an OpenAPI schema to a CodegenProperty.
func (p *Parser) schemaToProperty(name string, schema *openapi3.Schema, required bool) *codegen.CodegenProperty {
	prop := &codegen.CodegenProperty{
		Name:                 p.toVarName(name),
		BaseName:             name,
		Required:             required,
		Deprecated:           schema.Deprecated,
		IsReadOnly:           schema.ReadOnly,
		IsWriteOnly:          schema.WriteOnly,
		IsNullable:           schema.Nullable,
		Description:          escapeUnsafeChars(schema.Description),
		UnescapedDescription: escapeUnsafeChars(schema.Description),
		Title:                schema.Title,
		Example:              fmt.Sprintf("%v", schema.Example),
		VendorExtensions:     convertExtensions(schema.Extensions),
	}

	// Set name variants
	prop.NameInLowerCase = strings.ToLower(prop.Name)
	prop.NameInCamelCase = p.toVarName(name)
	prop.NameInPascalCase = p.toModelName(name)
	prop.NameInSnakeCase = toSnakeCase(name)
	prop.HasSanitizedName = prop.Name != name

	// Getters/setters
	prop.Getter = "get" + p.toModelName(name)
	prop.Setter = "set" + p.toModelName(name)

	// Get schema type
	schemaType := ""
	if schema.Type != nil && len(schema.Type.Slice()) > 0 {
		schemaType = schema.Type.Slice()[0]
	}
	prop.OpenApiType = schemaType

	// Handle enums
	if len(schema.Enum) > 0 {
		prop.IsEnum = true
		prop.IsInnerEnum = true
		prop.AllowableValues = map[string]any{
			"values": schema.Enum,
		}
		// isString records whether each enum value must be quoted as a string literal.
		// It is stored on the enumVar itself rather than relying on Mustache context
		// fallback to the enclosing property: an array-of-enum parameter is not itself a
		// string, so the values of its inherited enumVars would otherwise render unquoted.
		isStringEnum := schemaType == "string"
		enumVars := make([]map[string]any, 0, len(schema.Enum))
		for _, v := range schema.Enum {
			// Escape single quotes for TypeScript string literals
			valueStr := fmt.Sprintf("%v", v)
			escapedValue := strings.ReplaceAll(valueStr, "'", "\\'")
			enumVars = append(enumVars, map[string]any{
				"name":     toEnumVarName(valueStr),
				"value":    escapedValue,
				"isString": isStringEnum,
			})
		}
		prop.AllowableValues["enumVars"] = enumVars
		prop.EnumName = p.toEnumName(name)
		prop.DatatypeWithEnum = prop.EnumName
	}

	// Type-specific handling
	switch schemaType {
	case "array":
		prop.IsArray = true
		prop.IsContainer = true
		prop.ContainerType = "array"
		if schema.Items != nil {
			// Check if items has a $ref
			if schema.Items.Ref != "" {
				refName := extractRefName(schema.Items.Ref)
				modelName := p.toModelName(refName)
				prop.Items = &codegen.CodegenProperty{
					DataType: modelName,
					Datatype: modelName,
					IsModel:  true,
				}
				prop.DataType = "Array<" + modelName + ">"
				prop.Datatype = "Array<" + modelName + ">"
				prop.BaseType = modelName
				prop.ComplexType = modelName
			} else if schema.Items.Value != nil {
				prop.Items = p.schemaToProperty(name+"Item", schema.Items.Value, false)
				prop.DataType = "Array<" + prop.Items.DataType + ">"
				prop.Datatype = "Array<" + prop.Items.DataType + ">"
				prop.BaseType = prop.Items.DataType
				if prop.Items.IsModel {
					prop.ComplexType = prop.Items.DataType
				}
				// Composite (union) items have no per-element (de)serializer;
				// treat them as primitive so elements are passed through instead
				// of referencing an undefined <union>FromJSON helper.
				if prop.Items.IsFreeFormObject && !prop.Items.IsPrimitiveType {
					prop.Items.IsPrimitiveType = true
				}
			} else {
				prop.DataType = "Array<any>"
				prop.Datatype = "Array<any>"
				prop.BaseType = "any"
			}
		} else {
			prop.DataType = "Array<any>"
			prop.Datatype = "Array<any>"
			prop.BaseType = "any"
		}
		prop.UniqueItems = schema.UniqueItems

		// When the array items are an enum, surface the enum on the array itself so a named
		// enum is generated and the element type references it. DataType stays the underlying
		// "Array<...>" while DatatypeWithEnum carries the enum element type, mirroring the
		// scalar-enum convention.
		if prop.Items != nil && prop.Items.IsEnum {
			prop.IsEnum = true
			prop.EnumName = p.toEnumName(name)
			prop.AllowableValues = prop.Items.AllowableValues
			prop.DatatypeWithEnum = "Array<" + prop.EnumName + ">"
		}

	case "object":
		switch {
		case len(schema.Properties) > 0:
			// For inline objects with properties, we don't generate a model
			// so treat them as "any" for simplicity
			prop.DataType = "any"
			prop.BaseType = "any"
			prop.IsPrimitiveType = true
			prop.IsFreeFormObject = true
		case schema.AdditionalProperties.Schema != nil:
			// Typed map: `additionalProperties` is a schema (possibly a $ref),
			// e.g. {[key: string]: boolean} or {[key: string]: SomeModel}.
			prop.IsMap = true
			prop.IsContainer = true
			prop.ContainerType = "map"
			prop.Items = p.schemaRefToProperty("value", schema.AdditionalProperties.Schema, false)
			prop.DataType = "{ [key: string]: " + prop.Items.DataType + "; }"
			prop.BaseType = prop.Items.DataType
			prop.IsPrimitiveType = prop.Items.IsPrimitiveType
			if prop.Items.IsModel {
				prop.ComplexType = prop.Items.BaseType
			}
		case schema.AdditionalProperties.Has != nil && *schema.AdditionalProperties.Has:
			// Free-form map: `additionalProperties: true`.
			prop.IsMap = true
			prop.IsContainer = true
			prop.ContainerType = "map"
			prop.DataType = "{ [key: string]: any; }"
			prop.BaseType = "any"
			prop.IsPrimitiveType = true
			prop.IsFreeFormObject = true
		default:
			prop.IsFreeFormObject = true
			prop.DataType = "any"
			prop.IsPrimitiveType = true
		}

	case "string":
		prop.IsString = true
		prop.IsPrimitiveType = true
		switch schema.Format {
		case "date":
			prop.IsDate = true
			prop.DataType = p.getSchemaType("string", "date")
		case "date-time":
			prop.IsDateTime = true
			prop.DataType = p.getSchemaType("string", "date-time")
		case "uuid":
			prop.IsUuid = true
			prop.DataType = "string"
		case "uri":
			prop.IsUri = true
			prop.DataType = "string"
		case "email":
			prop.IsEmail = true
			prop.DataType = "string"
		case "password":
			prop.IsPassword = true
			prop.DataType = "string"
		case "binary":
			prop.IsBinary = true
			prop.IsFile = true
			prop.DataType = "Blob"
			prop.IsPrimitiveType = false
		case "byte":
			prop.IsByteArray = true
			prop.DataType = "string"
		default:
			prop.DataType = "string"
		}

	case "integer":
		prop.IsInteger = true
		prop.IsNumeric = true
		prop.IsPrimitiveType = true
		if schema.Format == "int64" {
			prop.IsLong = true
		}
		prop.DataType = "number"

	case "number":
		prop.IsNumber = true
		prop.IsNumeric = true
		prop.IsPrimitiveType = true
		switch schema.Format {
		case "float":
			prop.IsFloat = true
		case "double":
			prop.IsDouble = true
		}
		prop.DataType = "number"

	case "boolean":
		prop.IsBoolean = true
		prop.IsPrimitiveType = true
		prop.DataType = "boolean"

	default:
		// No explicit "type": resolve composition (allOf/oneOf/anyOf) to the
		// member type(s) that are available instead of collapsing to `any`.
		resolved := false
		switch {
		case len(schema.AllOf) == 1:
			resolved = p.applyMemberType(prop, schema.AllOf[0])
		case len(schema.AllOf) > 1:
			resolved = p.applyCompositeType(prop, schema.AllOf, " & ")
		case len(schema.OneOf) == 1:
			resolved = p.applyMemberType(prop, schema.OneOf[0])
		case len(schema.OneOf) > 1:
			resolved = p.applyCompositeType(prop, schema.OneOf, " | ")
		case len(schema.AnyOf) == 1:
			resolved = p.applyMemberType(prop, schema.AnyOf[0])
		case len(schema.AnyOf) > 1:
			resolved = p.applyCompositeType(prop, schema.AnyOf, " | ")
		}
		if !resolved {
			prop.DataType = p.getSchemaType(schemaType, schema.Format)
			if prop.DataType != "any" && prop.DataType != "" {
				prop.IsModel = true
			}
		}
	}

	// Handle $ref - this would need to look at the schema reference
	if prop.DataType == "" {
		prop.DataType = "any"
		prop.IsAnyType = true
	}

	// Sync Datatype with DataType for template compatibility
	prop.Datatype = prop.DataType

	// Also sync Items.Datatype if present
	if prop.Items != nil && prop.Items.Datatype == "" {
		prop.Items.Datatype = prop.Items.DataType
	}

	// Set DatatypeWithEnum if not already set
	if prop.DatatypeWithEnum == "" {
		prop.DatatypeWithEnum = prop.DataType
	}

	// Validation
	prop.Pattern = schema.Pattern
	if schema.Min != nil {
		prop.Minimum = fmt.Sprintf("%v", *schema.Min)
	}
	if schema.Max != nil {
		prop.Maximum = fmt.Sprintf("%v", *schema.Max)
	}
	prop.MinLength = intPtr(int(schema.MinLength))
	prop.MaxLength = uint64ToIntPtr(schema.MaxLength)
	prop.MinItems = intPtr(int(schema.MinItems))
	prop.MaxItems = uint64ToIntPtr(schema.MaxItems)
	prop.ExclusiveMinimum = schema.ExclusiveMin
	prop.ExclusiveMaximum = schema.ExclusiveMax
	prop.HasValidation = prop.Pattern != "" || prop.Minimum != "" || prop.Maximum != "" ||
		prop.MinLength != nil || prop.MaxLength != nil ||
		prop.MinItems != nil || prop.MaxItems != nil

	// Default value
	if schema.Default != nil {
		prop.DefaultValue = fmt.Sprintf("%v", schema.Default)
	}

	// Set lowercase datatype alias for templates
	prop.Datatype = prop.DataType

	// IsDateType/IsDateTimeType mirror IsDate/IsDateTime. The typescript-fetch model
	// template keys date (de)serialization (new Date(...) / .toISOString()) off the
	// *Type variants, so without this the generated FromJSON/ToJSON pass date fields
	// through as raw ISO strings while still typing them as Date.
	prop.IsDateType = prop.IsDate
	prop.IsDateTimeType = prop.IsDateTime

	return prop
}

// isObjectSchema reports whether the schema's primary type is "object".
func isObjectSchema(schema *openapi3.Schema) bool {
	return schema.Type != nil && len(schema.Type.Slice()) > 0 && schema.Type.Slice()[0] == "object"
}
