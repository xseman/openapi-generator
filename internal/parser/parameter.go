package parser

import (
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/xseman/openapi-generator/internal/codegen"
)

// parameterToCodegen converts an OpenAPI parameter to a CodegenParameter.
func (p *Parser) parameterToCodegen(param *openapi3.Parameter) *codegen.CodegenParameter {
	cp := &codegen.CodegenParameter{
		BaseName:             param.Name,
		ParamName:            p.toVarName(param.Name),
		Required:             param.Required,
		Description:          escapeUnsafeChars(param.Description),
		UnescapedDescription: escapeUnsafeChars(param.Description),
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
