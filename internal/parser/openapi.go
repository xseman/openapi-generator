// Package parser provides OpenAPI specification parsing functionality.
package parser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/xseman/openapi-generator/internal/codegen"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Parser parses OpenAPI specifications and converts them to codegen models.
type Parser struct {
	// The loaded OpenAPI document
	Doc *openapi3.T

	// Generator for type conversions
	TypeMapping     map[string]string
	GetTypeFunc     func(schemaType, format string) string
	ToModelNameFunc func(name string) string
	ToVarNameFunc   func(name string) string

	// Validation settings
	SkipValidation bool

	// Collected validation errors and warnings
	ValidationErrors   []string
	ValidationWarnings []string
}

// NewParser creates a new OpenAPI parser.
func NewParser() *Parser {
	return &Parser{
		TypeMapping: make(map[string]string),
	}
}

// LoadFromFile loads an OpenAPI spec from a file.
func (p *Parser) LoadFromFile(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Read file to detect version
	data, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Check if it's Swagger 2.0
	if isSwagger2(data) {
		return p.loadSwagger2FromData(data)
	}

	// Load as OpenAPI 3.x
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to load OpenAPI spec: %w", err)
	}

	p.Doc = doc

	// Validate the spec (unless skipped)
	return p.validateSpec()
}

// LoadFromURL loads an OpenAPI spec from a URL.
func (p *Parser) LoadFromURL(urlStr string) error {
	u, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("failed to parse URL: %w", err)
	}

	// Note: For URLs, we attempt OpenAPI 3 first, then fall back to Swagger 2 on error
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromURI(u)
	if err != nil {
		return fmt.Errorf("failed to load OpenAPI spec from URL: %w", err)
	}

	p.Doc = doc

	// Validate the spec (unless skipped)
	return p.validateSpec()
}

// LoadFromData loads an OpenAPI spec from raw data.
func (p *Parser) LoadFromData(data []byte) error {
	// Check if it's Swagger 2.0
	if isSwagger2(data) {
		return p.loadSwagger2FromData(data)
	}

	// Load as OpenAPI 3.x
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(data)
	if err != nil {
		return fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	p.Doc = doc

	// Validate the spec (unless skipped)
	return p.validateSpec()
}

// isSwagger2 checks if the data represents a Swagger 2.0 specification.
func isSwagger2(data []byte) bool {
	// Simple check: look for "swagger": "2.0" in the JSON/YAML
	var temp struct {
		Swagger string `json:"swagger" yaml:"swagger"`
	}

	// Try JSON first
	if err := json.Unmarshal(data, &temp); err == nil {
		return strings.HasPrefix(temp.Swagger, "2.")
	}

	return false
}

// loadSwagger2FromData loads a Swagger 2.0 spec and converts it to OpenAPI 3.
func (p *Parser) loadSwagger2FromData(data []byte) error {
	var doc2 openapi2.T

	if err := json.Unmarshal(data, &doc2); err != nil {
		return fmt.Errorf("failed to parse Swagger 2.0 spec: %w", err)
	}

	// Convert to OpenAPI 3
	doc3, err := openapi2conv.ToV3(&doc2)
	if err != nil {
		return fmt.Errorf("failed to convert Swagger 2.0 to OpenAPI 3: %w", err)
	}

	// Fix missing PathItem-level parameters
	// The openapi2conv library doesn't properly convert PathItem.Parameters to operations
	p.fixPathItemParameters(&doc2, doc3)

	p.Doc = doc3

	// Note: Validation with converted specs may have issues
	// Validation will still happen if SkipValidation is false, but we accept the risk
	return p.validateSpec()
}

// fixPathItemParameters copies PathItem-level parameters from Swagger 2 to OpenAPI 3 operations.
// The openapi2conv library has a bug where it doesn't copy PathItem.Parameters to operations.
func (p *Parser) fixPathItemParameters(doc2 *openapi2.T, doc3 *openapi3.T) {
	if doc3.Paths == nil {
		return
	}

	for path, v2PathItem := range doc2.Paths {
		if len(v2PathItem.Parameters) == 0 {
			continue
		}

		// Get the converted v3 pathItem
		v3PathItem := doc3.Paths.Value(path)
		if v3PathItem == nil {
			continue
		}

		// Convert v2 parameter refs to v3 parameter refs
		var v3Params openapi3.Parameters
		for _, v2Param := range v2PathItem.Parameters {
			if v2Param.Ref != "" {
				// Convert ref format from #/parameters/name to #/components/parameters/name
				v3Ref := convertParameterRef(v2Param.Ref)
				v3Params = append(v3Params, &openapi3.ParameterRef{
					Ref: v3Ref,
				})
			} else {
				// Convert inline parameter
				paramRef, _, _, _ := openapi2conv.ToV3Parameter(doc3.Components, v2Param, doc2.Consumes)
				if paramRef != nil {
					v3Params = append(v3Params, paramRef)
				}
			}
		}

		// Add parameters to all operations in this pathItem
		for _, op := range []*openapi3.Operation{
			v3PathItem.Get, v3PathItem.Post, v3PathItem.Put, v3PathItem.Delete,
			v3PathItem.Patch, v3PathItem.Options, v3PathItem.Head,
		} {
			if op != nil {
				// Create a set of existing parameter refs and names to avoid duplicates
				existingRefs := make(map[string]bool)
				existingNames := make(map[string]bool)

				for _, param := range op.Parameters {
					if param.Ref != "" {
						existingRefs[param.Ref] = true
					} else if param.Value != nil {
						key := param.Value.In + ":" + param.Value.Name
						existingNames[key] = true
					}
				}

				// Only add PathItem parameters that don't already exist
				for _, pathParam := range v3Params {
					shouldAdd := false

					if pathParam.Ref != "" {
						shouldAdd = !existingRefs[pathParam.Ref]
					} else if pathParam.Value != nil {
						key := pathParam.Value.In + ":" + pathParam.Value.Name
						shouldAdd = !existingNames[key]
					}

					if shouldAdd {
						op.Parameters = append([]*openapi3.ParameterRef{pathParam}, op.Parameters...)
					}
				}
			}
		}
	}
}

// convertParameterRef converts a Swagger 2.0 parameter ref to OpenAPI 3 format.
func convertParameterRef(v2Ref string) string {
	// #/parameters/name -> #/components/parameters/name
	if strings.HasPrefix(v2Ref, "#/parameters/") {
		return strings.Replace(v2Ref, "#/parameters/", "#/components/parameters/", 1)
	}
	return v2Ref
}

// validateSpec validates the OpenAPI specification.
// It collects errors and warnings, and either returns an error or logs warnings
// depending on the SkipValidation flag.
func (p *Parser) validateSpec() error {
	if p.Doc == nil {
		return fmt.Errorf("no document loaded")
	}

	ctx := context.Background()
	err := p.Doc.Validate(ctx)

	// Collect validation errors
	if err != nil {
		p.ValidationErrors = append(p.ValidationErrors, err.Error())
	}

	// Collect warnings about unused schemas
	p.collectWarnings()

	// If there are validation errors
	if len(p.ValidationErrors) > 0 {
		if p.SkipValidation {
			// Log warnings but don't fail
			fmt.Fprintf(os.Stderr, "There were issues with the specification, but validation has been explicitly disabled.\n")
			fmt.Fprintf(os.Stderr, "Errors:\n")
			for _, msg := range p.ValidationErrors {
				fmt.Fprintf(os.Stderr, "  - %s\n", msg)
			}
			if len(p.ValidationWarnings) > 0 {
				fmt.Fprintf(os.Stderr, "Warnings:\n")
				for _, msg := range p.ValidationWarnings {
					fmt.Fprintf(os.Stderr, "  - %s\n", msg)
				}
			}
			return nil
		}

		// Fail with detailed error message
		var sb strings.Builder
		sb.WriteString("There were issues with the specification. The option can be disabled via --skip-validate-spec (CLI).\n")
		sb.WriteString("Errors:\n")
		for _, msg := range p.ValidationErrors {
			sb.WriteString(fmt.Sprintf("  - %s\n", msg))
		}
		if len(p.ValidationWarnings) > 0 {
			sb.WriteString("Warnings:\n")
			for _, msg := range p.ValidationWarnings {
				sb.WriteString(fmt.Sprintf("  - %s\n", msg))
			}
		}
		return errors.New(sb.String())
	}

	return nil
}

// collectWarnings collects warnings about the spec (e.g., unused schemas).
func (p *Parser) collectWarnings() {
	if p.Doc == nil || p.Doc.Components == nil || p.Doc.Components.Schemas == nil {
		return
	}

	// Find unused schemas
	usedSchemas := make(map[string]bool)

	// Mark schemas used in paths
	if p.Doc.Paths != nil {
		for path := range p.Doc.Paths.Map() {
			pathItem := p.Doc.Paths.Value(path)
			if pathItem != nil {
				p.markSchemasInPathItem(pathItem, usedSchemas)
			}
		}
	}

	// Check for unused schemas
	for schemaName := range p.Doc.Components.Schemas {
		if !usedSchemas[schemaName] {
			p.ValidationWarnings = append(p.ValidationWarnings, fmt.Sprintf("Unused model: %s", schemaName))
		}
	}
}

// markSchemasInPathItem marks all schemas referenced in a path item as used.
func (p *Parser) markSchemasInPathItem(pathItem *openapi3.PathItem, usedSchemas map[string]bool) {
	operations := []*openapi3.Operation{
		pathItem.Get, pathItem.Post, pathItem.Put, pathItem.Delete,
		pathItem.Patch, pathItem.Options, pathItem.Head,
	}

	for _, op := range operations {
		if op == nil {
			continue
		}

		// Mark schemas in request body
		if op.RequestBody != nil && op.RequestBody.Value != nil {
			for _, content := range op.RequestBody.Value.Content {
				if content.Schema != nil {
					p.markSchemaAsUsed(content.Schema, usedSchemas)
				}
			}
		}

		// Mark schemas in parameters
		for _, param := range op.Parameters {
			if param != nil && param.Value != nil && param.Value.Schema != nil {
				p.markSchemaAsUsed(param.Value.Schema, usedSchemas)
			}
		}

		// Mark schemas in responses
		if op.Responses != nil {
			for _, response := range op.Responses.Map() {
				if response != nil && response.Value != nil {
					for _, content := range response.Value.Content {
						if content.Schema != nil {
							p.markSchemaAsUsed(content.Schema, usedSchemas)
						}
					}
				}
			}
		}
	}
}

// markSchemaAsUsed recursively marks a schema and its references as used.
func (p *Parser) markSchemaAsUsed(schemaRef *openapi3.SchemaRef, usedSchemas map[string]bool) {
	if schemaRef == nil {
		return
	}

	// If it's a reference, extract the schema name
	if schemaRef.Ref != "" {
		// Extract name from #/components/schemas/Name
		parts := strings.Split(schemaRef.Ref, "/")
		if len(parts) > 0 {
			schemaName := parts[len(parts)-1]

			// Check if already marked to prevent infinite recursion
			if usedSchemas[schemaName] {
				return
			}

			usedSchemas[schemaName] = true

			// Recursively check referenced schema
			if p.Doc.Components != nil && p.Doc.Components.Schemas != nil {
				if refSchema := p.Doc.Components.Schemas[schemaName]; refSchema != nil && refSchema.Value != nil {
					p.markSchemaPropertiesAsUsed(refSchema.Value, usedSchemas)
				}
			}
		}
		return
	}

	// Check the schema value itself
	if schemaRef.Value != nil {
		p.markSchemaPropertiesAsUsed(schemaRef.Value, usedSchemas)
	}
}

// markSchemaPropertiesAsUsed marks schemas referenced in properties, items, etc.
func (p *Parser) markSchemaPropertiesAsUsed(schema *openapi3.Schema, usedSchemas map[string]bool) {
	if schema == nil {
		return
	}

	// Check properties
	for _, propRef := range schema.Properties {
		p.markSchemaAsUsed(propRef, usedSchemas)
	}

	// Check items (for arrays)
	if schema.Items != nil {
		p.markSchemaAsUsed(schema.Items, usedSchemas)
	}

	// Check additionalProperties
	if schema.AdditionalProperties.Schema != nil {
		p.markSchemaAsUsed(schema.AdditionalProperties.Schema, usedSchemas)
	}

	// Check allOf, anyOf, oneOf
	for _, s := range schema.AllOf {
		p.markSchemaAsUsed(s, usedSchemas)
	}
	for _, s := range schema.AnyOf {
		p.markSchemaAsUsed(s, usedSchemas)
	}
	for _, s := range schema.OneOf {
		p.markSchemaAsUsed(s, usedSchemas)
	}
}

// GetInfo returns basic info about the API.
func (p *Parser) GetInfo() map[string]string {
	if p.Doc == nil || p.Doc.Info == nil {
		return nil
	}
	info := make(map[string]string)
	info["title"] = p.Doc.Info.Title
	info["description"] = p.Doc.Info.Description
	info["version"] = p.Doc.Info.Version
	if p.Doc.Info.TermsOfService != "" {
		info["termsOfService"] = p.Doc.Info.TermsOfService
	}
	if p.Doc.Info.Contact != nil {
		if p.Doc.Info.Contact.Email != "" {
			info["infoEmail"] = p.Doc.Info.Contact.Email
		}
		if p.Doc.Info.Contact.URL != "" {
			info["infoUrl"] = p.Doc.Info.Contact.URL
		}
	}
	if p.Doc.Info.License != nil {
		info["licenseName"] = p.Doc.Info.License.Name
		info["licenseUrl"] = p.Doc.Info.License.URL
	}
	return info
}

// GetBasePath returns the base path from servers.
func (p *Parser) GetBasePath() string {
	if p.Doc == nil || len(p.Doc.Servers) == 0 {
		return ""
	}
	return p.Doc.Servers[0].URL
}

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

	return models, nil
}

// GetOperations extracts all operations grouped by tag.
func (p *Parser) GetOperations() (map[string][]*codegen.CodegenOperation, error) {
	if p.Doc == nil || p.Doc.Paths == nil {
		return nil, nil
	}

	operationsByTag := make(map[string][]*codegen.CodegenOperation)

	// Get paths in sorted order
	pathNames := make([]string, 0, p.Doc.Paths.Len())
	for path := range p.Doc.Paths.Map() {
		pathNames = append(pathNames, path)
	}
	sort.Strings(pathNames)

	for _, path := range pathNames {
		pathItem := p.Doc.Paths.Value(path)
		if pathItem == nil {
			continue
		}

		// Process each HTTP method
		methods := map[string]*openapi3.Operation{
			"GET":     pathItem.Get,
			"POST":    pathItem.Post,
			"PUT":     pathItem.Put,
			"DELETE":  pathItem.Delete,
			"PATCH":   pathItem.Patch,
			"OPTIONS": pathItem.Options,
			"HEAD":    pathItem.Head,
		}

		for method, op := range methods {
			if op == nil {
				continue
			}

			operation := p.operationToCodegen(path, method, op, pathItem.Parameters)

			// Determine tag
			tag := "default"
			if len(op.Tags) > 0 {
				tag = op.Tags[0]
			}

			operationsByTag[tag] = append(operationsByTag[tag], operation)
		}
	}

	return operationsByTag, nil
}

// GetSecuritySchemes extracts security schemes.
func (p *Parser) GetSecuritySchemes() ([]*codegen.CodegenSecurity, error) {
	if p.Doc == nil || p.Doc.Components == nil || p.Doc.Components.SecuritySchemes == nil {
		return nil, nil
	}

	var schemes []*codegen.CodegenSecurity

	for name, schemeRef := range p.Doc.Components.SecuritySchemes {
		if schemeRef == nil || schemeRef.Value == nil {
			continue
		}

		scheme := p.securitySchemeToCodegen(name, schemeRef.Value)
		schemes = append(schemes, scheme)
	}

	return schemes, nil
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
		Description:          schema.Description,
		UnescapedDescription: schema.Description,
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
				model.AdditionalPropertiesType = p.getTypeDeclaration(schema.AdditionalProperties.Schema.Value)
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

	// Handle composition
	if len(schema.OneOf) > 0 {
		model.OneOf = make([]string, 0, len(schema.OneOf))
		oneOfModelsMap := make(map[string]bool) // Use map for deduplication
		for _, ref := range schema.OneOf {
			if ref.Ref != "" {
				refName := extractRefName(ref.Ref)
				modelName := p.toModelName(refName)
				model.OneOf = append(model.OneOf, modelName)
				// Add non-primitive models to OneOfModels for import generation
				if !isPrimitiveType(modelName) {
					oneOfModelsMap[modelName] = true
				}
			} else if ref.Value != nil {
				typeName := p.getTypeDeclaration(ref.Value)
				model.OneOf = append(model.OneOf, typeName)
				// Only add non-primitive types
				if !isPrimitiveType(typeName) {
					oneOfModelsMap[typeName] = true
				}
			}
		}
		// Convert map to sorted slice
		model.OneOfModels = make([]string, 0, len(oneOfModelsMap))
		for modelName := range oneOfModelsMap {
			model.OneOfModels = append(model.OneOfModels, modelName)
		}
		sort.Strings(model.OneOfModels)
		model.HasOneOf = len(model.OneOf) > 0
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
			PropertyName:     schema.Discriminator.PropertyName,
			PropertyBaseName: schema.Discriminator.PropertyName,
			Mapping:          schema.Discriminator.Mapping,
		}
		if len(schema.Discriminator.Mapping) > 0 {
			model.HasDiscriminatorWithNonEmptyMapping = true
			model.Discriminator.MappedModels = make([]*codegen.MappedModel, 0)
			for mappingName, schemaRef := range schema.Discriminator.Mapping {
				model.Discriminator.MappedModels = append(model.Discriminator.MappedModels, &codegen.MappedModel{
					MappingName: mappingName,
					ModelName:   extractRefName(schemaRef),
				})
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

// extractProperties extracts properties from an object schema.
func (p *Parser) extractProperties(schema *openapi3.Schema, model *codegen.CodegenModel) []*codegen.CodegenProperty {
	if schema.Properties == nil {
		return nil
	}

	requiredSet := make(map[string]bool)
	for _, r := range schema.Required {
		requiredSet[r] = true
	}

	// Sort property names
	propNames := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		propNames = append(propNames, name)
	}
	sort.Strings(propNames)

	var props []*codegen.CodegenProperty
	for _, name := range propNames {
		propRef := schema.Properties[name]
		if propRef == nil || propRef.Value == nil {
			continue
		}

		required := requiredSet[name]
		prop := p.schemaToProperty(name, propRef.Value, required)
		props = append(props, prop)
	}

	return props
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
		Description:          schema.Description,
		UnescapedDescription: schema.Description,
		Title:                schema.Title,
		Example:              fmt.Sprintf("%v", schema.Example),
		VendorExtensions:     convertExtensions(schema.Extensions),
	}

	// Set name variants
	prop.NameInLowerCase = strings.ToLower(prop.Name)
	prop.NameInCamelCase = p.toVarName(name)
	prop.NameInPascalCase = p.toModelName(name)
	prop.NameInSnakeCase = toSnakeCase(name)

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
		enumVars := make([]map[string]any, 0, len(schema.Enum))
		for _, v := range schema.Enum {
			// Escape single quotes for TypeScript string literals
			valueStr := fmt.Sprintf("%v", v)
			escapedValue := strings.ReplaceAll(valueStr, "'", "\\'")
			enumVars = append(enumVars, map[string]any{
				"name":  toEnumVarName(valueStr),
				"value": escapedValue,
			})
		}
		prop.AllowableValues["enumVars"] = enumVars
		prop.EnumName = p.toModelName(name) + "Enum"
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

	case "object":
		if schema.Properties != nil {
			// For inline objects with properties, we don't generate a model
			// so treat them as "any" for simplicity
			prop.DataType = "any"
			prop.BaseType = "any"
			prop.IsPrimitiveType = true
			prop.IsFreeFormObject = true
		} else if schema.AdditionalProperties.Has != nil && *schema.AdditionalProperties.Has {
			prop.IsMap = true
			prop.IsContainer = true
			prop.ContainerType = "map"
			if schema.AdditionalProperties.Schema != nil {
				prop.Items = p.schemaToProperty("value", schema.AdditionalProperties.Schema.Value, false)
				prop.DataType = "{ [key: string]: " + prop.Items.DataType + "; }"
				prop.BaseType = prop.Items.DataType
				prop.IsPrimitiveType = prop.Items.IsPrimitiveType
			} else {
				prop.DataType = "{ [key: string]: any; }"
				prop.BaseType = "any"
				prop.IsPrimitiveType = true
			}
			prop.IsFreeFormObject = true
		} else {
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
		if schema.Format == "float" {
			prop.IsFloat = true
		} else if schema.Format == "double" {
			prop.IsDouble = true
		}
		prop.DataType = "number"

	case "boolean":
		prop.IsBoolean = true
		prop.IsPrimitiveType = true
		prop.DataType = "boolean"

	default:
		// Check for $ref
		prop.DataType = p.getSchemaType(schemaType, schema.Format)
		if prop.DataType != "any" && prop.DataType != "" {
			prop.IsModel = true
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

	return prop
}

// operationToCodegen converts an OpenAPI operation to a CodegenOperation.
func (p *Parser) operationToCodegen(path, method string, op *openapi3.Operation, pathParams openapi3.Parameters) *codegen.CodegenOperation {
	co := &codegen.CodegenOperation{
		Path:                path,
		HttpMethod:          method,
		OperationId:         op.OperationID,
		OperationIdOriginal: op.OperationID,
		Summary:             op.Summary,
		Notes:               op.Description,
		UnescapedNotes:      op.Description,
		IsDeprecated:        op.Deprecated,
		VendorExtensions:    convertExtensions(op.Extensions),
	}

	// Generate operation ID if not provided
	if co.OperationId == "" {
		co.OperationId = strings.ToLower(method) + sanitizeTag(path)
	}

	// Set operation ID variants
	co.OperationIdCamelCase = toCamelCase(co.OperationId)
	co.OperationIdLowerCase = strings.ToLower(co.OperationId)
	co.OperationIdSnakeCase = toSnakeCase(co.OperationId)
	co.Nickname = co.OperationIdCamelCase

	// Set tag/baseName
	if len(op.Tags) > 0 {
		co.BaseName = op.Tags[0]
	} else {
		co.BaseName = "default"
	}

	// Process path parameters from path item
	for _, paramRef := range pathParams {
		if paramRef == nil || paramRef.Value == nil {
			continue
		}
		param := p.parameterToCodegen(paramRef.Value)
		co.AllParams = append(co.AllParams, param)
		co.PathParams = append(co.PathParams, param)
	}

	// Process operation parameters
	for _, paramRef := range op.Parameters {
		if paramRef == nil || paramRef.Value == nil {
			continue
		}
		param := p.parameterToCodegen(paramRef.Value)
		co.AllParams = append(co.AllParams, param)

		switch paramRef.Value.In {
		case "path":
			co.PathParams = append(co.PathParams, param)
		case "query":
			co.QueryParams = append(co.QueryParams, param)
		case "header":
			co.HeaderParams = append(co.HeaderParams, param)
		case "cookie":
			co.CookieParams = append(co.CookieParams, param)
		}

		if param.Required {
			co.RequiredParams = append(co.RequiredParams, param)
		} else {
			co.OptionalParams = append(co.OptionalParams, param)
			co.HasOptionalParams = true
		}
	}

	// Process request body
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		body := op.RequestBody.Value
		for contentType, mediaType := range body.Content {
			if mediaType.Schema != nil && mediaType.Schema.Value != nil {
				bodyParam := &codegen.CodegenParameter{
					ParamName:   "body",
					BaseName:    "body",
					IsBodyParam: true,
					Required:    body.Required,
					Description: body.Description,
					ContentType: contentType,
				}

				schema := mediaType.Schema.Value
				if mediaType.Schema.Ref != "" {
					refName := extractRefName(mediaType.Schema.Ref)
					modelName := p.toModelName(refName)
					// Use "any" if model name is empty
					if modelName == "" {
						modelName = "any"
					}
					bodyParam.DataType = modelName
					bodyParam.BaseType = modelName
					bodyParam.IsModel = true
				} else {
					bodyParam.DataType = p.getTypeDeclaration(schema)
					bodyParam.BaseType = bodyParam.DataType
				}

				// Use "any" if type declaration is empty
				if bodyParam.DataType == "" {
					bodyParam.DataType = "any"
					bodyParam.BaseType = "any"
				}

				co.BodyParam = bodyParam
				co.BodyParams = append(co.BodyParams, bodyParam)
				co.AllParams = append(co.AllParams, bodyParam)

				// Add to required/optional params
				if bodyParam.Required {
					co.RequiredParams = append(co.RequiredParams, bodyParam)
				} else {
					co.OptionalParams = append(co.OptionalParams, bodyParam)
					co.HasOptionalParams = true
				}

				// Check for multipart
				if strings.HasPrefix(contentType, "multipart/") {
					co.IsMultipart = true
				}

				break // Use first content type
			}
		}
	}

	// Process responses
	if op.Responses != nil {
		for code, respRef := range op.Responses.Map() {
			if respRef == nil || respRef.Value == nil {
				continue
			}

			resp := p.responseToCodegen(code, respRef.Value)
			co.Responses = append(co.Responses, resp)

			// Set return type from 2xx response
			if strings.HasPrefix(code, "2") && resp.DataType != "" {
				co.ReturnType = resp.DataType
				co.ReturnBaseType = resp.BaseType
				co.ReturnSimpleType = resp.SimpleType
				co.ReturnTypeIsPrimitive = resp.PrimitiveType
				if resp.IsArray {
					co.IsArray = true
					co.ReturnContainer = "array"
				}
				if resp.IsMap {
					co.IsMap = true
					co.ReturnContainer = "map"
				}
				if resp.IsBinary || resp.IsFile {
					co.IsResponseBinary = true
					co.IsResponseFile = resp.IsFile
				}
			}
		}
	}

	// Set content types
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		for ct := range op.RequestBody.Value.Content {
			co.Consumes = append(co.Consumes, map[string]string{"mediaType": ct})
		}
		co.HasConsumes = len(co.Consumes) > 0
	}

	// Process security
	if op.Security != nil {
		for _, secReq := range *op.Security {
			for name, scopes := range secReq {
				sec := &codegen.CodegenSecurity{
					Name:   name,
					Scopes: make([]map[string]any, len(scopes)),
				}
				for i, scope := range scopes {
					sec.Scopes[i] = map[string]any{"scope": scope}
				}
				co.AuthMethods = append(co.AuthMethods, sec)
			}
		}
		co.HasAuthMethods = len(co.AuthMethods) > 0
	}

	// Deduplicate parameters by name (path params can override operation params)
	co.AllParams = deduplicateParams(co.AllParams)
	co.PathParams = deduplicateParams(co.PathParams)
	co.QueryParams = deduplicateParams(co.QueryParams)
	co.HeaderParams = deduplicateParams(co.HeaderParams)

	// Rebuild required and optional params from deduplicated allParams
	co.RequiredParams = nil
	co.OptionalParams = nil
	co.HasOptionalParams = false
	for _, param := range co.AllParams {
		if param.Required {
			co.RequiredParams = append(co.RequiredParams, param)
		} else {
			co.OptionalParams = append(co.OptionalParams, param)
			co.HasOptionalParams = true
		}
	}

	// Collect imports
	co.Imports = p.collectOperationImports(co)

	return co
}

// parameterToCodegen converts an OpenAPI parameter to a CodegenParameter.
func (p *Parser) parameterToCodegen(param *openapi3.Parameter) *codegen.CodegenParameter {
	cp := &codegen.CodegenParameter{
		BaseName:             param.Name,
		ParamName:            p.toVarName(param.Name),
		Required:             param.Required,
		Description:          param.Description,
		UnescapedDescription: param.Description,
		IsDeprecated:         param.Deprecated,
		Style:                string(param.Style),
		IsExplode:            param.Explode != nil && *param.Explode,
		VendorExtensions:     convertExtensions(param.Extensions),
	}

	// Set name variants
	cp.NameInLowerCase = strings.ToLower(cp.ParamName)
	cp.NameInCamelCase = cp.ParamName
	cp.NameInPascalCase = p.toModelName(param.Name)
	cp.NameInSnakeCase = toSnakeCase(param.Name)

	// Set location flags
	switch param.In {
	case "path":
		cp.IsPathParam = true
	case "query":
		cp.IsQueryParam = true
	case "header":
		cp.IsHeaderParam = true
	case "cookie":
		cp.IsCookieParam = true
	}

	// Process schema
	if param.Schema != nil && param.Schema.Value != nil {
		schema := param.Schema.Value
		prop := p.schemaToProperty(param.Name, schema, param.Required)

		cp.DataType = prop.DataType
		cp.BaseType = prop.BaseType
		cp.DataFormat = schema.Format
		cp.IsArray = prop.IsArray
		cp.IsMap = prop.IsMap
		cp.IsString = prop.IsString
		cp.IsInteger = prop.IsInteger
		cp.IsLong = prop.IsLong
		cp.IsNumber = prop.IsNumber
		cp.IsFloat = prop.IsFloat
		cp.IsDouble = prop.IsDouble
		cp.IsBoolean = prop.IsBoolean
		cp.IsDate = prop.IsDate
		cp.IsDateTime = prop.IsDateTime
		cp.IsEnum = prop.IsEnum
		cp.IsPrimitiveType = prop.IsPrimitiveType
		cp.IsModel = prop.IsModel
		cp.IsContainer = prop.IsContainer
		cp.Items = prop.Items
		cp.AllowableValues = prop.AllowableValues
		cp.EnumName = prop.EnumName
		cp.DatatypeWithEnum = prop.DatatypeWithEnum

		// Collection format
		if cp.IsArray {
			cp.CollectionFormat = "multi"
			cp.IsCollectionFormatMulti = true
		}
	}

	// Example
	if param.Example != nil {
		cp.Example = fmt.Sprintf("%v", param.Example)
	}

	return cp
}

// responseToCodegen converts an OpenAPI response to a CodegenResponse.
func (p *Parser) responseToCodegen(code string, resp *openapi3.Response) *codegen.CodegenResponse {
	desc := ptrString(resp.Description)
	cr := &codegen.CodegenResponse{
		Code:                 code,
		Message:              desc,
		Description:          desc,
		UnescapedDescription: desc,
		VendorExtensions:     convertExtensions(resp.Extensions),
	}

	// Set status code categories
	if code == "default" {
		cr.IsDefault = true
	} else if strings.HasPrefix(code, "1") {
		cr.Is1xx = true
	} else if strings.HasPrefix(code, "2") {
		cr.Is2xx = true
	} else if strings.HasPrefix(code, "3") {
		cr.Is3xx = true
	} else if strings.HasPrefix(code, "4") {
		cr.Is4xx = true
	} else if strings.HasPrefix(code, "5") {
		cr.Is5xx = true
	}

	// Process content
	for _, mediaType := range resp.Content {
		if mediaType.Schema == nil {
			continue
		}

		if mediaType.Schema.Ref != "" {
			refName := extractRefName(mediaType.Schema.Ref)
			modelName := p.toModelName(refName)
			if modelName == "" {
				modelName = "any"
			}
			cr.DataType = modelName
			cr.BaseType = modelName
			cr.IsModel = true
		} else if mediaType.Schema.Value != nil {
			schema := mediaType.Schema.Value
			prop := p.schemaToProperty("response", schema, false)

			cr.DataType = prop.DataType
			cr.BaseType = prop.BaseType
			cr.IsArray = prop.IsArray
			cr.IsMap = prop.IsMap
			cr.IsModel = prop.IsModel
			cr.IsBinary = prop.IsBinary
			cr.IsFile = prop.IsFile
			cr.IsPrimitiveType = prop.IsPrimitiveType
			cr.IsString = prop.IsString
			cr.IsInteger = prop.IsInteger
			cr.IsNumber = prop.IsNumber
			cr.IsBoolean = prop.IsBoolean
			cr.Items = prop.Items
			cr.ContainerType = prop.ContainerType

			cr.SimpleType = prop.IsPrimitiveType
			cr.PrimitiveType = prop.IsPrimitiveType
		}

		break // Use first content type
	}

	// Process headers
	for name, headerRef := range resp.Headers {
		if headerRef == nil || headerRef.Value == nil {
			continue
		}
		header := headerRef.Value
		prop := &codegen.CodegenProperty{
			Name:        name,
			BaseName:    name,
			Description: header.Description,
		}
		if header.Schema != nil && header.Schema.Value != nil {
			prop.DataType = p.getTypeDeclaration(header.Schema.Value)
		}
		cr.Headers = append(cr.Headers, prop)
	}
	cr.HasHeaders = len(cr.Headers) > 0

	return cr
}

// securitySchemeToCodegen converts an OpenAPI security scheme to CodegenSecurity.
func (p *Parser) securitySchemeToCodegen(name string, scheme *openapi3.SecurityScheme) *codegen.CodegenSecurity {
	cs := &codegen.CodegenSecurity{
		Name:             name,
		Description:      scheme.Description,
		Type:             scheme.Type,
		Scheme:           scheme.Scheme,
		VendorExtensions: convertExtensions(scheme.Extensions),
	}

	switch scheme.Type {
	case "apiKey":
		cs.IsApiKey = true
		cs.KeyParamName = scheme.Name
		switch scheme.In {
		case "query":
			cs.IsKeyInQuery = true
		case "header":
			cs.IsKeyInHeader = true
		case "cookie":
			cs.IsKeyInCookie = true
		}

	case "http":
		cs.IsBasic = true
		switch strings.ToLower(scheme.Scheme) {
		case "basic":
			cs.IsBasicBasic = true
		case "bearer":
			cs.IsBasicBearer = true
			cs.BearerFormat = scheme.BearerFormat
		}

	case "oauth2":
		cs.IsOAuth = true
		if scheme.Flows != nil {
			if scheme.Flows.AuthorizationCode != nil {
				cs.IsCode = true
				cs.AuthorizationUrl = scheme.Flows.AuthorizationCode.AuthorizationURL
				cs.TokenUrl = scheme.Flows.AuthorizationCode.TokenURL
				cs.RefreshUrl = scheme.Flows.AuthorizationCode.RefreshURL
				cs.Scopes = scopesToList(scheme.Flows.AuthorizationCode.Scopes)
			} else if scheme.Flows.Implicit != nil {
				cs.IsImplicit = true
				cs.AuthorizationUrl = scheme.Flows.Implicit.AuthorizationURL
				cs.Scopes = scopesToList(scheme.Flows.Implicit.Scopes)
			} else if scheme.Flows.Password != nil {
				cs.IsPassword = true
				cs.TokenUrl = scheme.Flows.Password.TokenURL
				cs.Scopes = scopesToList(scheme.Flows.Password.Scopes)
			} else if scheme.Flows.ClientCredentials != nil {
				cs.IsApplication = true
				cs.TokenUrl = scheme.Flows.ClientCredentials.TokenURL
				cs.Scopes = scopesToList(scheme.Flows.ClientCredentials.Scopes)
			}
		}
		cs.HasScopes = len(cs.Scopes) > 0

	case "openIdConnect":
		cs.IsOpenId = true
		cs.OpenIdConnectUrl = scheme.OpenIdConnectUrl
	}

	return cs
}

// Helper functions

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

func (p *Parser) collectImports(model *codegen.CodegenModel) []string {
	imports := make(map[string]bool)

	for _, prop := range model.Vars {
		// Skip self-references to avoid circular imports
		if prop.IsModel && prop.DataType != model.Classname && !isPrimitiveType(prop.DataType) {
			imports[prop.DataType] = true
		}
		// Also check items for arrays
		if prop.Items != nil && prop.Items.IsModel && prop.Items.DataType != model.Classname && !isPrimitiveType(prop.Items.DataType) {
			imports[prop.Items.DataType] = true
		}
	}

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
		if param.IsModel && !isPrimitiveType(param.DataType) {
			imports[param.DataType] = true
		}
		if param.Items != nil && param.Items.IsModel && !isPrimitiveType(param.Items.DataType) {
			imports[param.Items.DataType] = true
		}
	}

	// From return type - also check that ReturnBaseType is not primitive
	if op.ReturnType != "" && !isPrimitiveType(op.ReturnType) && !isPrimitiveType(op.ReturnBaseType) {
		imports[op.ReturnBaseType] = true
	}

	result := make([]string, 0, len(imports))
	for imp := range imports {
		result = append(result, imp)
	}
	sort.Strings(result)
	return result
}

// Utility functions

func extractRefName(ref string) string {
	parts := strings.Split(ref, "/")
	return parts[len(parts)-1]
}

func convertExtensions(ext map[string]any) map[string]any {
	if ext == nil {
		return make(map[string]any)
	}
	return ext
}

// deduplicateParams removes duplicate parameters by name, keeping the last occurrence.
// This is useful when path-level and operation-level parameters have the same name.
func deduplicateParams(params []*codegen.CodegenParameter) []*codegen.CodegenParameter {
	if len(params) == 0 {
		return params
	}

	seen := make(map[string]int)
	var result []*codegen.CodegenParameter

	// Iterate through parameters and track positions
	for i, param := range params {
		if param == nil {
			continue
		}
		key := param.ParamName
		if idx, exists := seen[key]; exists {
			// Replace previous occurrence
			result[idx] = param
		} else {
			// First occurrence
			seen[key] = len(result)
			result = append(result, param)
		}
		_ = i // unused
	}

	return result
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

func intPtr(i int) *int {
	if i == 0 {
		return nil
	}
	return &i
}

func uint64ToIntPtr(v *uint64) *int {
	if v == nil {
		return nil
	}
	i := int(*v)
	return &i
}

func ptrString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func toCamelCase(s string) string {
	words := splitWords(s)
	if len(words) == 0 {
		return s
	}
	titleCaser := cases.Title(language.English)
	result := strings.ToLower(words[0])
	for _, word := range words[1:] {
		result += titleCaser.String(strings.ToLower(word))
	}
	return result
}

func toPascalCase(s string) string {
	words := splitWords(s)
	titleCaser := cases.Title(language.English)
	result := ""
	for _, word := range words {
		result += titleCaser.String(strings.ToLower(word))
	}
	return result
}

func toSnakeCase(s string) string {
	words := splitWords(s)
	result := ""
	for i, word := range words {
		if i > 0 {
			result += "_"
		}
		result += strings.ToLower(word)
	}
	return result
}

func splitWords(s string) []string {
	// Split on non-alphanumeric characters and camelCase boundaries
	var words []string
	var current strings.Builder

	for i, r := range s {
		if !isAlphanumeric(r) {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
			continue
		}

		if i > 0 && isUpperCase(r) && current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}

		current.WriteRune(r)
	}

	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}

func isAlphanumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func isUpperCase(r rune) bool {
	return r >= 'A' && r <= 'Z'
}

func sanitizeTag(path string) string {
	// Convert path to a valid identifier
	path = strings.ReplaceAll(path, "/", "_")
	path = strings.ReplaceAll(path, "{", "")
	path = strings.ReplaceAll(path, "}", "")
	path = strings.ReplaceAll(path, "-", "_")
	return toPascalCase(path)
}

func toEnumVarName(value string) string {
	// Convert enum value to a valid enum member name
	name := strings.ToUpper(value)
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, "+", "PLUS")

	// Handle names starting with a digit - prefix with underscore
	if len(name) > 0 && name[0] >= '0' && name[0] <= '9' {
		name = "_" + name
	}

	// Remove any remaining invalid characters
	validName := ""
	for i, r := range name {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '_' || (i > 0 && r >= '0' && r <= '9') {
			validName += string(r)
		}
	}
	if validName == "" {
		validName = "VALUE"
	}
	return validName
}

func isPrimitiveType(t string) bool {
	primitives := map[string]bool{
		"string": true, "number": true, "boolean": true,
		"any": true, "void": true, "null": true,
		"Date": true, "Blob": true,
	}
	return primitives[t]
}

func scopesToList(scopes map[string]string) []map[string]any {
	result := make([]map[string]any, 0, len(scopes))
	for scope, desc := range scopes {
		result = append(result, map[string]any{
			"scope":       scope,
			"description": desc,
		})
	}
	return result
}
