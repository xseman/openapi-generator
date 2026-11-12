// Package dart provides Dart-specific code generation utilities.
package dart

import (
	"maps"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/xseman/openapi-generator/internal/codegen"
	"github.com/xseman/openapi-generator/internal/config"
	"github.com/xseman/openapi-generator/internal/generator"
)

// TypeMapping maps OpenAPI types to the TypeScript-shaped DataType strings
// the parser builds on (Array<X>, any, number): it tells a model from a
// primitive by them. Dart types are derived from a property's flags and
// tree instead, see DartType.
var TypeMapping = map[string]string{
	"array":   "Array",
	"boolean": "boolean",
	"integer": "number",
	"number":  "number",
	"object":  "any",
	"string":  "string",
	"AnyType": "any",
}

// ReservedWords is the set of Dart reserved words and built-in identifiers
// that must not be used as bare variable, parameter, or field names.
var ReservedWords = map[string]bool{
	// Reserved words (cannot be used as identifiers at all)
	"assert": true, "break": true, "case": true, "catch": true,
	"class": true, "const": true, "continue": true, "default": true,
	"do": true, "else": true, "enum": true, "extends": true,
	"false": true, "final": true, "finally": true, "for": true,
	"if": true, "in": true, "is": true, "new": true,
	"null": true, "rethrow": true, "return": true, "super": true,
	"switch": true, "this": true, "throw": true, "true": true,
	"try": true, "var": true, "void": true, "while": true,
	"with": true,
	// Built-in identifiers (cannot be used as type names)
	"abstract": true, "as": true, "covariant": true, "deferred": true,
	"dynamic": true, "export": true, "extension": true, "external": true,
	"factory": true, "function": true, "get": true, "implements": true,
	"import": true, "interface": true, "late": true, "library": true,
	"mixin": true, "operator": true, "part": true, "required": true,
	"set": true, "static": true, "typedef": true,
	// Async-related (contextually reserved)
	"async": true, "await": true, "yield": true, "sync": true,
	// Built-in types
	"int": true, "double": true, "num": true, "bool": true,
	"string": true, "list": true, "map": true, "object": true,
	"future": true, "stream": true, "iterable": true,
	// Local variables used in API method templates
	"queryParameters": true, "headerParams": true, "formParams": true,
	"requestOptions": true, "varLocalPath": true,
}

// Primitives is the set of Dart primitive type names.
var Primitives = map[string]bool{
	"int": true, "double": true, "num": true, "bool": true,
	"String": true, "Object": true, "DateTime": true, "Duration": true,
	"List": true, "Map": true, "Set": true, "Iterable": true,
	"Future": true, "Stream": true, "dynamic": true,
}

// BaseGenerator is the base generator for Dart languages.
// It mirrors AbstractDartCodegen in the Java implementation.
type BaseGenerator struct {
	// Configuration
	Config     *config.GeneratorConfig
	DartConfig *config.DartFetchConfig

	// Type mappings
	TypeMapping                map[string]string
	ImportMapping              map[string]string
	ReservedWords              map[string]bool
	LanguageSpecificPrimitives map[string]bool
	InstantiationTypes         map[string]string

	// Name mappings
	NameMapping          map[string]string
	ParameterNameMapping map[string]string
	ModelNameMapping     map[string]string
	EnumNameMapping      map[string]string

	// Package configuration
	ModelPackage string
	ApiPackage   string
	SourceDir    string

	// Naming configuration
	ModelNamePrefix string
	ModelNameSuffix string
	ApiNameSuffix   string

	// Template files
	ApiTemplateFiles   map[string]string
	ModelTemplateFiles map[string]string
	SupportingFiles    []generator.SupportingFile

	// Additional properties for templates
	AdditionalProperties map[string]any

	// Behavior flags
	ModelPropertyNaming config.ModelPropertyNamingType
}

// NewBaseGenerator creates a new Dart base generator.
func NewBaseGenerator() *BaseGenerator {
	g := &BaseGenerator{
		TypeMapping:                maps.Clone(TypeMapping),
		ReservedWords:              maps.Clone(ReservedWords),
		LanguageSpecificPrimitives: maps.Clone(Primitives),
		ImportMapping:              make(map[string]string),
		InstantiationTypes:         make(map[string]string),
		NameMapping:                make(map[string]string),
		ParameterNameMapping:       make(map[string]string),
		ModelNameMapping:           make(map[string]string),
		EnumNameMapping:            make(map[string]string),
		ApiTemplateFiles:           make(map[string]string),
		ModelTemplateFiles:         make(map[string]string),
		SupportingFiles:            make([]generator.SupportingFile, 0),
		AdditionalProperties:       make(map[string]any),
		ApiNameSuffix:              "Api",
		ModelPropertyNaming:        config.PropertyNamingCamelCase,
	}

	return g
}

// GetTypeMapping returns the type mapping.
func (g *BaseGenerator) GetTypeMapping() map[string]string { return g.TypeMapping }

// GetReservedWords returns the reserved words.
func (g *BaseGenerator) GetReservedWords() map[string]bool { return g.ReservedWords }

// GetLanguageSpecificPrimitives returns primitive types.
func (g *BaseGenerator) GetLanguageSpecificPrimitives() map[string]bool {
	return g.LanguageSpecificPrimitives
}

// GetImportMapping returns the import mapping.
func (g *BaseGenerator) GetImportMapping() map[string]string { return g.ImportMapping }

// GetAdditionalProperties returns additional properties for templates.
func (g *BaseGenerator) GetAdditionalProperties() map[string]any { return g.AdditionalProperties }

// IsReservedWord checks if a word is reserved (case-insensitive lookup).
func (g *BaseGenerator) IsReservedWord(word string) bool {
	return g.ReservedWords[strings.ToLower(word)]
}

// IsPrimitive checks if a type is a Dart primitive.
func (g *BaseGenerator) IsPrimitive(typeName string) bool {
	return g.LanguageSpecificPrimitives[typeName]
}

// EscapeReservedWord escapes a reserved word by suffixing it with an underscore,
// matching common Dart practice and avoiding the leading-underscore privacy rule.
func (g *BaseGenerator) EscapeReservedWord(name string) string {
	if g.IsReservedWord(name) {
		return name + "_"
	}

	return name
}

// GetSchemaType returns the parser's DataType string for an OpenAPI type.
func (g *BaseGenerator) GetSchemaType(schemaType, _ string) string {
	return g.TypeMapping[schemaType]
}

// GetTypeDeclaration returns the parser's DataType string for an OpenAPI type.
func (g *BaseGenerator) GetTypeDeclaration(schemaType, format string) string {
	return g.GetSchemaType(schemaType, format)
}

// ToModelName converts a schema name to a Dart model class name (PascalCase).
func (g *BaseGenerator) ToModelName(name string) string {
	if mapped, ok := g.ModelNameMapping[name]; ok {
		return mapped
	}

	result := name
	if g.ModelNamePrefix != "" {
		result = g.ModelNamePrefix + result
	}

	if g.ModelNameSuffix != "" {
		result += g.ModelNameSuffix
	}

	return g.toDartTypeName(result, "Model")
}

// ToApiName converts a tag to a Dart API class name (PascalCase + ApiNameSuffix).
func (g *BaseGenerator) ToApiName(name string) string {
	return Camelize(SanitizeName(name), false) + g.ApiNameSuffix
}

// ToVarName converts a property name to a Dart variable name.
//
// Dart convention is camelCase for fields, parameters, and local variables.
// Reserved words are escaped via EscapeReservedWord. Names starting with a
// digit are prefixed with "n".
func (g *BaseGenerator) ToVarName(name string) string {
	name = SanitizeName(name)

	// Preserve dunder-style names (e.g. "_id") only if they round-trip; Dart
	// treats a leading underscore as library-private, so we strip it for fields.
	name = strings.TrimLeft(name, "_")
	if name == "" {
		return "value"
	}

	var converted string

	switch g.ModelPropertyNaming {
	case config.PropertyNamingOriginal:
		converted = name
	case config.PropertyNamingCamelCase:
		converted = Camelize(name, true)
	case config.PropertyNamingPascalCase:
		converted = Camelize(name, false)
	case config.PropertyNamingSnakeCase:
		converted = Underscore(name)
	default:
		converted = Camelize(name, true)
	}

	if converted == "" {
		return "value"
	}

	if unicode.IsDigit(rune(converted[0])) {
		converted = "n" + converted
	}

	return g.EscapeReservedWord(converted)
}

// ToParamName converts a parameter name to a Dart identifier.
func (g *BaseGenerator) ToParamName(name string) string {
	return g.ToVarName(name)
}

// SanitizeOperationId sanitizes an operation ID.
func (g *BaseGenerator) SanitizeOperationId(operationId string) string {
	return SanitizeName(operationId)
}

// toDartTypeName converts a name to a valid Dart type name.
func (g *BaseGenerator) toDartTypeName(name, safePrefix string) string {
	name = regexp.MustCompile(`[^\w| ]`).ReplaceAllString(name, "")
	name = Camelize(name, false)

	if g.IsReservedWord(name) {
		return safePrefix + name
	}

	if name != "" && unicode.IsDigit(rune(name[0])) {
		return safePrefix + name
	}

	if g.IsPrimitive(name) {
		return safePrefix + name
	}

	return name
}

// ToModelFilename returns the model file name in Dart snake_case convention.
func (g *BaseGenerator) ToModelFilename(name string) string {
	return Underscore(name)
}

// ToApiFilename returns the API file name in Dart snake_case convention.
func (g *BaseGenerator) ToApiFilename(name string) string {
	return Underscore(name)
}

// FromModel converts an OpenAPI schema to a CodegenModel (placeholder).
func (g *BaseGenerator) FromModel(name string, _ any) *codegen.CodegenModel {
	cm := &codegen.CodegenModel{
		Name:          g.EscapeReservedWord(name),
		SchemaName:    name,
		Classname:     g.ToModelName(name),
		ClassVarName:  g.ToVarName(name),
		ClassFilename: g.ToModelFilename(name),
	}

	return cm
}

// FromOperation converts an OpenAPI operation to a CodegenOperation (placeholder).
func (g *BaseGenerator) FromOperation(path, httpMethod string, _ any) *codegen.CodegenOperation {
	return &codegen.CodegenOperation{
		Path:       path,
		HttpMethod: strings.ToUpper(httpMethod),
	}
}

// FromProperty converts an OpenAPI property to a CodegenProperty (placeholder).
func (g *BaseGenerator) FromProperty(name string, _ any, required bool) *codegen.CodegenProperty {
	return &codegen.CodegenProperty{
		Name:     g.ToVarName(name),
		BaseName: name,
		Required: required,
	}
}

// FromParameter converts an OpenAPI parameter to a CodegenParameter (placeholder).
func (g *BaseGenerator) FromParameter(_ any) *codegen.CodegenParameter {
	return &codegen.CodegenParameter{}
}

// FromResponse converts an OpenAPI response to a CodegenResponse (placeholder).
func (g *BaseGenerator) FromResponse(code string, _ any) *codegen.CodegenResponse {
	return &codegen.CodegenResponse{Code: code}
}

// FromSecurityScheme converts a security scheme to CodegenSecurity (placeholder).
func (g *BaseGenerator) FromSecurityScheme(name string, _ any) *codegen.CodegenSecurity {
	return &codegen.CodegenSecurity{Name: name}
}

// PostProcessModels post-processes models (default: identity).
func (g *BaseGenerator) PostProcessModels(models []*codegen.CodegenModel) []*codegen.CodegenModel {
	return models
}

// PostProcessOperations post-processes operations (default: identity).
func (g *BaseGenerator) PostProcessOperations(ops []*codegen.CodegenOperation) []*codegen.CodegenOperation {
	return ops
}

// GetSupportingFiles returns supporting files.
func (g *BaseGenerator) GetSupportingFiles() []generator.SupportingFile { return g.SupportingFiles }

// GetApiTemplateFiles returns API template files.
func (g *BaseGenerator) GetApiTemplateFiles() map[string]string { return g.ApiTemplateFiles }

// GetModelTemplateFiles returns model template files.
func (g *BaseGenerator) GetModelTemplateFiles() map[string]string { return g.ModelTemplateFiles }

// GetConfig returns the generator config.
func (g *BaseGenerator) GetConfig() *config.GeneratorConfig { return g.Config }

// SetConfig sets the generator config.
func (g *BaseGenerator) SetConfig(cfg *config.GeneratorConfig) { g.Config = cfg }

// DartType is the Dart type of a property, from its flags and its Items
// tree. named is the class at its leaf: the model it refers to or the enum
// declared for its inline values; "" types the leaf by its flags, an inline
// object as a map and a union as Object?.
func DartType(p *codegen.CodegenProperty, named string) string {
	switch {
	case p == nil:
		return "Object?"
	case p.IsArray:
		return "List<" + itemType(p.Items, named) + ">"
	case p.IsMap:
		return "Map<String, " + itemType(p.Items, named) + ">"
	case named != "":
		return named
	case isComposite(p.DataType) || p.ComposedSchemas != nil:
		// ponytail: an inline union is the JSON value as decoded; the
		// upgrade is the sealed class a union model gets.
		return "Object?"
	}

	switch {
	case p.IsDate, p.IsDateTime:
		return "DateTime"
	case p.IsString:
		return "String"
	case p.IsInteger, p.IsLong:
		return "int"
	case p.IsNumber, p.IsFloat, p.IsDouble:
		return "double"
	case p.IsBoolean:
		return "bool"
	case p.OpenApiType == "object":
		return "Map<String, Object?>"
	}

	return "Object?"
}

// itemType is the Dart type of a list element or map value: DartType,
// nullable when the item is.
func itemType(p *codegen.CodegenProperty, named string) string {
	t := DartType(p, named)
	if p != nil && p.IsNullable {
		return Nullable(t)
	}

	return t
}

// Nullable is t with a question mark, once: Object? takes no second one.
func Nullable(t string) string {
	if strings.HasSuffix(t, "?") {
		return t
	}

	return t + "?"
}

// EnumBaseType is the Dart type an enum's values have: String when they
// are strings (isString on the enumVars), else bool, int when every value
// is whole, double otherwise; Object? when they are of more than one JSON
// type, which no enum's value field holds. A null among them makes the enum
// nullable, not mixed.
func EnumBaseType(allowable map[string]any, isBoolean bool) string {
	values, _ := allowable["values"].([]any)

	kinds := map[string]bool{}

	for _, v := range values {
		switch v.(type) {
		case nil:
		case string:
			kinds["string"] = true
		case bool:
			kinds["bool"] = true
		default:
			kinds["number"] = true
		}
	}

	if len(kinds) > 1 {
		return "Object?"
	}

	vars, _ := allowable["enumVars"].([]map[string]any)
	if len(vars) > 0 && vars[0]["isString"] == true {
		return "String"
	}

	if isBoolean {
		return "bool"
	}

	for _, v := range values {
		if f, ok := v.(float64); ok && f != float64(int64(f)) {
			return "double"
		}
	}

	return "int"
}

// isComposite reports whether the parser typed a value as a union or
// intersection of members ("A | B", "A & B").
func isComposite(dataType string) bool {
	return strings.Contains(dataType, " | ") || strings.Contains(dataType, " & ")
}

// Literal is s as a Dart string literal: a spec's text may hold a quote, a
// line break or a dollar sign, which Dart would interpolate.
func Literal(s string) string {
	return strings.ReplaceAll(strconv.Quote(s), "$", `\$`)
}

// Helpers

// SanitizeName replaces non-word characters with underscores.
func SanitizeName(name string) string {
	if name == "+1" {
		return "plus1"
	}

	if name == "-1" {
		return "minus1"
	}

	return regexp.MustCompile(`\W`).ReplaceAllString(name, "_")
}

// Camelize converts a string to camelCase or PascalCase.
//
// Words are split on non-alphanumeric characters and on lowercase->uppercase
// boundaries. When lowercaseFirst is true the first word's first character
// is lowercased; remaining words are capitalized.
func Camelize(s string, lowercaseFirst bool) string {
	if s == "" {
		return s
	}

	var (
		words   []string
		current strings.Builder
	)

	for i, r := range s {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if !isAlphaNum {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}

			continue
		}

		if i > 0 && r >= 'A' && r <= 'Z' && current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}

		current.WriteRune(r)
	}

	if current.Len() > 0 {
		words = append(words, current.String())
	}

	var out strings.Builder

	for i, w := range words {
		if w == "" {
			continue
		}

		if i == 0 && lowercaseFirst {
			out.WriteString(strings.ToLower(w[:1]) + w[1:])
		} else {
			out.WriteString(strings.ToUpper(w[:1]) + strings.ToLower(w[1:]))
		}
	}

	return out.String()
}

// Underscore converts a string to snake_case.
func Underscore(s string) string {
	// Replace non-alphanumeric with underscore first
	s = regexp.MustCompile(`\W`).ReplaceAllString(s, "_")
	// Insert underscore before uppercase letters preceded by lowercase or digit
	s = regexp.MustCompile(`([a-z0-9])([A-Z])`).ReplaceAllString(s, "${1}_${2}")
	// Collapse runs of underscores
	s = regexp.MustCompile(`_+`).ReplaceAllString(s, "_")

	return strings.ToLower(strings.Trim(s, "_"))
}
