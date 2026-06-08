// Package parser provides OpenAPI specification parsing functionality.
package parser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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

	// Fetch the URL content first to detect the version
	client := &http.Client{}
	resp, err := client.Get(urlStr)
	if err != nil {
		return fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check if it's Swagger 2.0 and use proper conversion
	if isSwagger2(data) {
		return p.loadSwagger2FromData(data)
	}

	// Load as OpenAPI 3.x
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
			fmt.Fprintf(&sb, "  - %s\n", msg)
		}
		if len(p.ValidationWarnings) > 0 {
			sb.WriteString("Warnings:\n")
			for _, msg := range p.ValidationWarnings {
				fmt.Fprintf(&sb, "  - %s\n", msg)
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

		// Process each HTTP method. Use a fixed-order slice rather than a map:
		// Go map iteration is randomised, which would otherwise shuffle the order
		// of generated methods within a path (and thus within each API class).
		methods := []struct {
			name string
			op   *openapi3.Operation
		}{
			{"GET", pathItem.Get},
			{"POST", pathItem.Post},
			{"PUT", pathItem.Put},
			{"DELETE", pathItem.Delete},
			{"PATCH", pathItem.Patch},
			{"OPTIONS", pathItem.Options},
			{"HEAD", pathItem.Head},
		}

		for _, m := range methods {
			method, op := m.name, m.op
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

	// Iterate scheme names in sorted order for stable output.
	names := make([]string, 0, len(p.Doc.Components.SecuritySchemes))
	for name := range p.Doc.Components.SecuritySchemes {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		schemeRef := p.Doc.Components.SecuritySchemes[name]
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
			// Iterate mapping keys in sorted order for stable output.
			mappingNames := make([]string, 0, len(schema.Discriminator.Mapping))
			for mappingName := range schema.Discriminator.Mapping {
				mappingNames = append(mappingNames, mappingName)
			}
			sort.Strings(mappingNames)
			for _, mappingName := range mappingNames {
				model.Discriminator.MappedModels = append(model.Discriminator.MappedModels, &codegen.MappedModel{
					MappingName: mappingName,
					ModelName:   extractRefName(schema.Discriminator.Mapping[mappingName]),
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
	names := p.memberTypeNames(refs)
	if len(names) == 0 {
		return false
	}
	joined := strings.Join(names, sep)
	prop.DataType = joined
	prop.BaseType = joined
	prop.DatatypeWithEnum = joined
	prop.IsPrimitiveType = false
	prop.IsFreeFormObject = true
	for _, n := range names {
		if !isPrimitiveType(n) {
			prop.ComposedModels = append(prop.ComposedModels, n)
		}
	}
	return true
}

// memberTypeNames resolves composition members to their TypeScript type strings,
// using the referenced model for $ref members and the recursively resolved type
// for inline members. Empty/any members are dropped and duplicates removed.
func (p *Parser) memberTypeNames(refs openapi3.SchemaRefs) []string {
	out := make([]string, 0, len(refs))
	seen := make(map[string]bool)
	for _, m := range refs {
		if m == nil {
			continue
		}
		var t string
		switch {
		case m.Ref != "":
			t = p.toModelName(extractRefName(m.Ref))
		case m.Value != nil:
			t = p.schemaToProperty("member", m.Value, false).DataType
		}
		if t == "" || t == "any" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
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

	// Set operation ID variants.
	// OperationIdCamelCase is UpperCamelCase (PascalCase) and is used for type-level
	// identifiers such as the request interface and inline parameter enums. Nickname is
	// lowerCamelCase and is used for the generated method names.
	co.OperationIdCamelCase = toPascalCase(co.OperationId)
	co.OperationIdLowerCase = strings.ToLower(co.OperationId)
	co.OperationIdSnakeCase = toSnakeCase(co.OperationId)
	co.Nickname = toCamelCase(co.OperationId)

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

		// Iterate content types in a deterministic order. Go map iteration is randomised,
		// which would otherwise make the chosen schema/content-type (and therefore the
		// generated output) unstable across runs.
		contentTypes := make([]string, 0, len(body.Content))
		for ct := range body.Content {
			contentTypes = append(contentTypes, ct)
		}
		sort.Strings(contentTypes)

		for _, contentType := range contentTypes {
			mediaType := body.Content[contentType]
			if mediaType.Schema == nil || mediaType.Schema.Value == nil {
				continue
			}
			schema := mediaType.Schema.Value

			// multipart/form-data and form-urlencoded bodies expand into one form
			// parameter per schema property (e.g. `file`), matching upstream
			// openapi-generator, rather than a single opaque `body` parameter.
			isForm := strings.HasPrefix(contentType, "multipart/") ||
				contentType == "application/x-www-form-urlencoded"
			if isForm && isObjectSchema(schema) && len(schema.Properties) > 0 {
				p.addFormParams(co, schema, contentType)
				if strings.HasPrefix(contentType, "multipart/") {
					co.IsMultipart = true
				}
				break
			}

			bodyParam := &codegen.CodegenParameter{
				IsBodyParam: true,
				Required:    body.Required,
				Description: body.Description,
				ContentType: contentType,
			}

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
				// Name a $ref body after the referenced schema (camelCased).
				bodyParam.BaseName = refName
				bodyParam.ParamName = p.toVarName(refName)
			} else {
				// Derive the full property shape so inline bodies (arrays, primitives,
				// maps) carry their container/item information instead of collapsing to a
				// bare type. Without this an array body renders as `Array` and an
				// undefined `ArrayToJSON` helper.
				prop := p.schemaToProperty("body", schema, body.Required)
				bodyParam.DataType = prop.DataType
				bodyParam.BaseType = prop.BaseType
				bodyParam.IsArray = prop.IsArray
				bodyParam.IsMap = prop.IsMap
				bodyParam.IsContainer = prop.IsContainer
				bodyParam.IsPrimitiveType = prop.IsPrimitiveType
				bodyParam.IsModel = prop.IsModel
				bodyParam.IsString = prop.IsString
				bodyParam.IsNumber = prop.IsNumber
				bodyParam.IsInteger = prop.IsInteger
				bodyParam.IsBoolean = prop.IsBoolean
				bodyParam.Items = prop.Items

				// Fall back to the legacy declaration for composed schemas (allOf/oneOf)
				// that schemaToProperty cannot resolve to a concrete model.
				if (bodyParam.DataType == "" || bodyParam.DataType == "any") && len(schema.AllOf) > 0 {
					bodyParam.DataType = p.getTypeDeclaration(schema)
					bodyParam.BaseType = bodyParam.DataType
					bodyParam.IsModel = !isPrimitiveType(bodyParam.DataType)
					bodyParam.IsPrimitiveType = isPrimitiveType(bodyParam.DataType)
				}

				// Name an inline body following upstream: an array of a model takes the
				// innermost item model name; an array of primitives or a map uses
				// "requestBody"; everything else (primitives, free-form objects) uses "body".
				baseName := "body"
				switch {
				case prop.IsArray:
					inner := prop.Items
					for inner != nil && inner.Items != nil {
						inner = inner.Items
					}
					if inner != nil && inner.IsModel && !isPrimitiveType(inner.DataType) {
						baseName = inner.DataType
					} else {
						baseName = "request_body"
					}
				case prop.IsMap:
					baseName = "request_body"
				}
				bodyParam.BaseName = baseName
				bodyParam.ParamName = p.toVarName(baseName)
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

			break // Use first (sorted) content type
		}
	}

	// Process responses
	if op.Responses != nil {
		// Iterate status codes in sorted order. Go map iteration is randomised,
		// which would otherwise shuffle the generated response list and make the
		// chosen 2xx return type unstable across runs.
		respMap := op.Responses.Map()
		codes := make([]string, 0, len(respMap))
		for code := range respMap {
			codes = append(codes, code)
		}
		sort.Strings(codes)

		for _, code := range codes {
			respRef := respMap[code]
			if respRef == nil || respRef.Value == nil {
				continue
			}

			resp := p.responseToCodegen(code, respRef.Value)
			co.Responses = append(co.Responses, resp)

			// Set return type from the first (lowest) 2xx response that carries
			// a body, e.g. prefer 200 over 201 when both are present.
			if co.ReturnType == "" && strings.HasPrefix(code, "2") && resp.DataType != "" {
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

	// Set content types in sorted order for stable output.
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		consumes := make([]string, 0, len(op.RequestBody.Value.Content))
		for ct := range op.RequestBody.Value.Content {
			consumes = append(consumes, ct)
		}
		sort.Strings(consumes)
		for _, ct := range consumes {
			co.Consumes = append(co.Consumes, map[string]string{"mediaType": ct})
		}
		co.HasConsumes = len(co.Consumes) > 0
	}

	// Process security. Each requirement is a map of scheme name -> scopes;
	// iterate names in sorted order so AuthMethods is stable across runs.
	if op.Security != nil {
		for _, secReq := range *op.Security {
			names := make([]string, 0, len(secReq))
			for name := range secReq {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				scopes := secReq[name]
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

// addFormParams expands a multipart/form-urlencoded request-body schema into one
// CodegenParameter per property, appending them to the operation's FormParams and
// AllParams. This mirrors upstream openapi-generator, which surfaces form fields (e.g. an
// uploaded `file`) as individual parameters rather than a single opaque body.
func (p *Parser) addFormParams(co *codegen.CodegenOperation, schema *openapi3.Schema, contentType string) {
	requiredSet := make(map[string]bool)
	for _, r := range schema.Required {
		requiredSet[r] = true
	}

	propNames := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		propNames = append(propNames, name)
	}
	sort.Strings(propNames)

	for _, name := range propNames {
		propRef := schema.Properties[name]
		if propRef == nil || propRef.Value == nil {
			continue
		}
		required := requiredSet[name]
		prop := p.schemaRefToProperty(name, propRef, required)

		fp := &codegen.CodegenParameter{
			BaseName:         name,
			ParamName:        prop.Name,
			IsFormParam:      true,
			Required:         required,
			Description:      prop.Description,
			ContentType:      contentType,
			DataType:         prop.DataType,
			DatatypeWithEnum: prop.DatatypeWithEnum,
			BaseType:         prop.BaseType,
			IsArray:          prop.IsArray,
			IsMap:            prop.IsMap,
			IsContainer:      prop.IsContainer,
			IsPrimitiveType:  prop.IsPrimitiveType,
			IsModel:          prop.IsModel,
			IsString:         prop.IsString,
			IsNumber:         prop.IsNumber,
			IsInteger:        prop.IsInteger,
			IsBoolean:        prop.IsBoolean,
			IsBinary:         prop.IsBinary,
			IsFile:           prop.IsFile,
			IsDate:           prop.IsDate,
			IsDateType:       prop.IsDate,
			IsDateTime:       prop.IsDateTime,
			IsDateTimeType:   prop.IsDateTime,
			IsEnum:           prop.IsEnum,
			Items:            prop.Items,
			AllowableValues:  prop.AllowableValues,
			EnumName:         prop.EnumName,
			UniqueItems:      prop.UniqueItems,
		}
		if fp.IsArray {
			fp.CollectionFormat = "csv"
		}

		co.FormParams = append(co.FormParams, fp)
		co.AllParams = append(co.AllParams, fp)
		if required {
			co.RequiredParams = append(co.RequiredParams, fp)
		} else {
			co.OptionalParams = append(co.OptionalParams, fp)
			co.HasOptionalParams = true
		}
	}
}

// isObjectSchema reports whether the schema's primary type is "object".
func isObjectSchema(schema *openapi3.Schema) bool {
	return schema.Type != nil && len(schema.Type.Slice()) > 0 && schema.Type.Slice()[0] == "object"
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
		Style:                param.Style,
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
		prop := p.schemaRefToProperty(param.Name, param.Schema, param.Required)

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
		cp.IsDateType = prop.IsDate
		cp.IsDateTime = prop.IsDateTime
		cp.IsDateTimeType = prop.IsDateTime
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

	// Ensure DataType is never empty - default to "any" if not set
	if cp.DataType == "" {
		cp.DataType = "any"
		cp.BaseType = "any"
		cp.IsPrimitiveType = true
		cp.IsAnyType = true
	}

	// Ensure DatatypeWithEnum is set
	if cp.DatatypeWithEnum == "" {
		cp.DatatypeWithEnum = cp.DataType
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

	// Process content. Iterate content types in sorted order so the "first"
	// content type chosen below is stable across runs (Go map iteration is
	// randomised).
	respContentTypes := make([]string, 0, len(resp.Content))
	for ct := range resp.Content {
		respContentTypes = append(respContentTypes, ct)
	}
	sort.Strings(respContentTypes)
	for _, ct := range respContentTypes {
		mediaType := resp.Content[ct]
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

			// PrimitiveType drives whether the operation deserializes with a per-element
			// FromJSON mapper. For array/map responses the relevant question is whether the
			// element type is primitive: a primitive element (e.g. Array<number>) needs no
			// mapper and must not reference an undefined numberFromJSON helper. SimpleType
			// is reserved for scalar primitives only.
			basePrimitive := prop.IsPrimitiveType
			if (prop.IsArray || prop.IsMap) && prop.Items != nil {
				basePrimitive = prop.Items.IsPrimitiveType
			}
			cr.SimpleType = prop.IsPrimitiveType && !prop.IsArray && !prop.IsMap
			cr.PrimitiveType = basePrimitive
		}

		break // Use first content type
	}

	// Process headers in sorted order for stable output.
	headerNames := make([]string, 0, len(resp.Headers))
	for name := range resp.Headers {
		headerNames = append(headerNames, name)
	}
	sort.Strings(headerNames)
	for _, name := range headerNames {
		headerRef := resp.Headers[name]
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
		// Ensure DataType is never empty
		if prop.DataType == "" {
			prop.DataType = "any"
		}
		prop.Datatype = prop.DataType
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
		// Member models of a union/intersection property type.
		for _, m := range prop.ComposedModels {
			if m != model.Classname && !isPrimitiveType(m) {
				imports[m] = true
			}
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

// toEnumVarName builds the member name (key) for a generated enum entry. TypeScript-fetch
// uses PascalCase member names (matching upstream openapi-generator's default
// ENUM_PROPERTY_NAMING of PascalCase), while the caller preserves the underlying string
// value verbatim — so CALL_CENTRUM yields the key CallCentrum mapped to 'CALL_CENTRUM'.
//
// It replicates camelize(underscore(value)): a separator is inserted at camelCase
// boundaries, the whole string is lower-cased so SCREAMING_SNAKE_CASE runs collapse into
// single words (the camelCase splitter would otherwise fragment them into individual
// letters, e.g. ODPOCTAR -> O,D,P,... -> ODPOCTAR), then each word is Title-cased.
func toEnumVarName(value string) string {
	// Spell out "+" so values such as "C+" survive the alphanumeric-only word split.
	value = strings.ReplaceAll(value, "+", "_plus")

	var b strings.Builder
	var prev rune
	for i, r := range value {
		if i > 0 && isUpperCase(r) && ((prev >= 'a' && prev <= 'z') || (prev >= '0' && prev <= '9')) {
			b.WriteByte('_')
		}
		b.WriteRune(r)
		prev = r
	}

	name := toPascalCase(strings.ToLower(b.String()))
	if name == "" {
		return "Empty"
	}
	// Ensure the result is a valid identifier start.
	if name[0] >= '0' && name[0] <= '9' {
		name = "_" + name
	}
	return name
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
	// Iterate scope names in sorted order for stable output.
	names := make([]string, 0, len(scopes))
	for scope := range scopes {
		names = append(names, scope)
	}
	sort.Strings(names)

	result := make([]map[string]any, 0, len(scopes))
	for _, scope := range names {
		result = append(result, map[string]any{
			"scope":       scope,
			"description": scopes[scope],
		})
	}
	return result
}
