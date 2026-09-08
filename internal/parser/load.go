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
	"strings"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
)

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
	p.buildPropertyOrder(data)

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
	p.buildPropertyOrder(data)

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
	p.buildPropertyOrder(data)

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
	p.buildSwagger2PropertyOrder(data)

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
