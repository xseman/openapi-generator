package parser

import (
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/xseman/openapi-generator/internal/codegen"
)

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

// scopesToList converts an OAuth2 flow's scope map to a slice of template-friendly
// entries, sorted by scope name for stable output.
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
