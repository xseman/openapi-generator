package parser

import (
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/xseman/openapi-generator/internal/codegen"
)

// responseToCodegen converts an OpenAPI response to a CodegenResponse.
func (p *Parser) responseToCodegen(code string, resp *openapi3.Response) *codegen.CodegenResponse {
	desc := escapeUnsafeChars(ptrString(resp.Description))
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
			cr.ComposedModels = prop.ComposedModels

			// Union/intersection responses have no single FromJSON helper; treat
			// them as simple so the template parses the JSON without a converter.
			// applyCompositeType marks these free-form without the primitive flag,
			// which also covers unions collapsed to a single member.
			if isCompositeType(cr.DataType) || (prop.IsFreeFormObject && !prop.IsPrimitiveType) {
				cr.SimpleType = true
				cr.PrimitiveType = true
			}
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
			Description: escapeUnsafeChars(header.Description),
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

// ptrString dereferences s, returning "" for a nil pointer.
func ptrString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
