// Package parser provides OpenAPI specification parsing functionality.
package parser

import (
	"github.com/getkin/kin-openapi/openapi3"
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

	// propOrder records the declaration order of object properties as they appear
	// in the raw spec source, keyed by the resolved *openapi3.Schema. kin-openapi
	// stores Schema.Properties as a Go map, which is inherently unordered; Java's
	// DefaultCodegen preserves spec order (it's backed by a LinkedHashMap), so this
	// is recovered separately by walking the raw document. See buildPropertyOrder.
	// A nil/missing entry means the order could not be recovered (e.g. a Swagger 2
	// source, or a schema pulled in via an external ref); callers fall back to
	// alphabetical order in that case.
	propOrder map[*openapi3.Schema][]string
}

// NewParser creates a new OpenAPI parser.
func NewParser() *Parser {
	return &Parser{
		TypeMapping: make(map[string]string),
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
