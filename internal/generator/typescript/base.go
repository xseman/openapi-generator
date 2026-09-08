// Package typescript provides TypeScript-specific code generation utilities.
package typescript

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/xseman/openapi-generator/internal/codegen"
	"github.com/xseman/openapi-generator/internal/config"
	"github.com/xseman/openapi-generator/internal/generator"
)

// TypeMapping maps OpenAPI types to TypeScript types
var TypeMapping = map[string]string{
	"Set":       "Set",
	"set":       "Set",
	"Array":     "Array",
	"array":     "Array",
	"boolean":   "boolean",
	"decimal":   "string",
	"string":    "string",
	"int":       "number",
	"float":     "number",
	"number":    "number",
	"long":      "number",
	"short":     "number",
	"char":      "string",
	"double":    "number",
	"object":    "any",
	"integer":   "number",
	"Map":       "any",
	"map":       "any",
	"date":      "string",
	"DateTime":  "string",
	"date-time": "string",
	"binary":    "any",
	"File":      "any",
	"file":      "any",
	"ByteArray": "string",
	"UUID":      "string",
	"URI":       "string",
	"Error":     "Error",
	"AnyType":   "any",
}

// ReservedWords is the set of TypeScript reserved words
var ReservedWords = map[string]bool{
	// Local variables used in API methods
	"varLocalPath": true, "queryParameters": true, "headerParams": true,
	"formParams": true, "useFormData": true, "varLocalDeferred": true,
	"requestOptions": true,

	// TypeScript reserved words
	"abstract": true, "await": true, "boolean": true, "break": true,
	"byte": true, "case": true, "catch": true, "char": true,
	"class": true, "const": true, "continue": true, "debugger": true,
	"default": true, "delete": true, "do": true, "double": true,
	"else": true, "enum": true, "export": true, "extends": true,
	"false": true, "final": true, "finally": true, "float": true,
	"for": true, "function": true, "goto": true, "if": true,
	"implements": true, "import": true, "in": true, "instanceof": true,
	"int": true, "interface": true, "let": true, "long": true,
	"native": true, "new": true, "null": true, "package": true,
	"private": true, "protected": true, "public": true, "return": true,
	"short": true, "static": true, "super": true, "switch": true,
	"synchronized": true, "this": true, "throw": true, "transient": true,
	"true": true, "try": true, "typeof": true, "var": true,
	"void": true, "volatile": true, "while": true, "with": true,
	"yield": true,

	// Browser built-in types that may conflict with model names
	"blob": true, "file": true, "date": true, "error": true,
	"map": true, "set": true, "array": true, "object": true,
}

// tsReservedKeywords is the set of TypeScript/ECMAScript reserved keywords that are not
// valid as bare identifiers and must be escaped in property/parameter names. It mirrors
// upstream openapi-generator's AbstractTypeScriptClientCodegen reservedWords. Unlike the
// broader ReservedWords map — which also guards model-name collisions with built-in types
// (File, Blob, ...) and API-method local variables — these are the only words ToVarName
// escapes, so a property or form field named "file" stays "file" while "export" becomes
// "_export".
var tsReservedKeywords = map[string]bool{
	"abstract": true, "await": true, "boolean": true, "break": true,
	"byte": true, "case": true, "catch": true, "char": true,
	"class": true, "const": true, "continue": true, "debugger": true,
	"default": true, "delete": true, "do": true, "double": true,
	"else": true, "enum": true, "export": true, "extends": true,
	"false": true, "final": true, "finally": true, "float": true,
	"for": true, "function": true, "goto": true, "if": true,
	"implements": true, "import": true, "in": true, "instanceof": true,
	"int": true, "interface": true, "let": true, "long": true,
	"native": true, "new": true, "null": true, "package": true,
	"private": true, "protected": true, "public": true, "return": true,
	"short": true, "static": true, "super": true, "switch": true,
	"synchronized": true, "this": true, "throw": true, "transient": true,
	"true": true, "try": true, "typeof": true, "var": true,
	"void": true, "volatile": true, "while": true, "with": true,
	"yield": true,
}

// Primitives is the set of TypeScript primitive types. It mirrors upstream
// openapi-generator's AbstractTypeScriptClientCodegen#languageSpecificPrimitives,
// including the built-in TypeScript utility types (Record, Partial, ...), so that
// a schema named e.g. "Record" is renamed (ModelRecord) instead of colliding with
// the language's own Record<K, V> type.
var Primitives = map[string]bool{
	"string": true, "String": true, "boolean": true, "Boolean": true,
	"Double": true, "Integer": true, "Long": true, "Float": true,
	"Object": true, "object": true, "Array": true, "ReadonlyArray": true,
	"Date": true, "number": true, "any": true, "File": true, "Error": true,
	"Map": true, "Set": true,

	// TypeScript utility types (lib.es5.d.ts and later)
	"Awaited": true, "Partial": true, "Required": true, "Readonly": true,
	"Record": true, "Pick": true, "Omit": true, "Exclude": true,
	"Extract": true, "NonNullable": true, "Parameters": true,
	"ConstructorParameters": true, "ReturnType": true, "InstanceType": true,
	"NoInfer": true, "ThisParameterType": true, "OmitThisParameter": true,
	"ThisType": true, "Uppercase": true, "Lowercase": true,
	"Capitalize": true, "Uncapitalize": true,
}

// BaseGenerator is the base generator for TypeScript languages.
// It mirrors AbstractTypeScriptClientCodegen in Java.
type BaseGenerator struct {
	// Configuration
	Config   *config.GeneratorConfig
	TSConfig *config.TypeScriptFetchConfig

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
	ModelPropertyNaming     config.ModelPropertyNamingType
	NullSafeAdditionalProps bool
	AllowUnicodeIdentifiers bool
}

// NewBaseGenerator creates a new TypeScript base generator
func NewBaseGenerator() *BaseGenerator {
	g := &BaseGenerator{
		TypeMapping:                copyMap(TypeMapping),
		ReservedWords:              copyMapBool(ReservedWords),
		LanguageSpecificPrimitives: copyMapBool(Primitives),
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

// GetTypeMapping returns the type mapping
func (g *BaseGenerator) GetTypeMapping() map[string]string {
	return g.TypeMapping
}

// GetReservedWords returns the reserved words
func (g *BaseGenerator) GetReservedWords() map[string]bool {
	return g.ReservedWords
}

// GetLanguageSpecificPrimitives returns primitive types
func (g *BaseGenerator) GetLanguageSpecificPrimitives() map[string]bool {
	return g.LanguageSpecificPrimitives
}

// GetImportMapping returns the import mapping
func (g *BaseGenerator) GetImportMapping() map[string]string {
	return g.ImportMapping
}

// GetAdditionalProperties returns additional properties for templates
func (g *BaseGenerator) GetAdditionalProperties() map[string]any {
	return g.AdditionalProperties
}

// IsReservedWord checks if a word is reserved
func (g *BaseGenerator) IsReservedWord(word string) bool {
	return g.ReservedWords[strings.ToLower(word)]
}

// IsPrimitive checks if a type is a language primitive
func (g *BaseGenerator) IsPrimitive(typeName string) bool {
	return g.LanguageSpecificPrimitives[typeName]
}

// EscapeReservedWord escapes a reserved word
func (g *BaseGenerator) EscapeReservedWord(name string) string {
	if g.IsReservedWord(name) {
		return "_" + name
	}
	return name
}

// GetSchemaType returns the TypeScript type for an OpenAPI schema type
func (g *BaseGenerator) GetSchemaType(schemaType, format string) string {
	// Check format-specific mappings first
	if format != "" {
		key := schemaType + ":" + format
		if mapped, ok := g.TypeMapping[key]; ok {
			return mapped
		}
		if mapped, ok := g.TypeMapping[format]; ok {
			return mapped
		}
	}

	// Fall back to type mapping
	if mapped, ok := g.TypeMapping[schemaType]; ok {
		return mapped
	}

	return schemaType
}

// GetTypeDeclaration returns the type declaration for a schema
func (g *BaseGenerator) GetTypeDeclaration(schemaType, format string) string {
	return g.GetSchemaType(schemaType, format)
}

// ToModelName converts a schema name to a TypeScript model name
func (g *BaseGenerator) ToModelName(name string) string {
	// Check model name mapping
	if mapped, ok := g.ModelNameMapping[name]; ok {
		return mapped
	}

	// Apply prefix and suffix
	result := name
	if g.ModelNamePrefix != "" {
		result = g.ModelNamePrefix + result
	}
	if g.ModelNameSuffix != "" {
		result = result + g.ModelNameSuffix
	}

	return g.toTypescriptTypeName(result, "Model")
}

// ToApiName converts a tag to an API class name
func (g *BaseGenerator) ToApiName(name string) string {
	return Camelize(SanitizeName(name), false) + g.ApiNameSuffix
}

// ToVarName converts a property name to a variable name
func (g *BaseGenerator) ToVarName(name string) string {
	// Translate a leading @ (e.g. JSON-LD "@type") to at_ so it stays distinct
	// from a sibling property of the same name, matching upstream openapi-generator.
	if strings.HasPrefix(name, "@") {
		name = "at_" + name[1:]
	}

	name = sanitizeIdentifierName(name)

	// Apply model property naming convention
	var result string
	switch g.ModelPropertyNaming {
	case config.PropertyNamingOriginal:
		result = name
	case config.PropertyNamingCamelCase:
		result = Camelize(name, true)
	case config.PropertyNamingPascalCase:
		result = Camelize(name, false)
	case config.PropertyNamingSnakeCase:
		result = Underscore(name)
	default:
		result = Camelize(name, true)
	}

	// Escape TypeScript reserved keywords and identifiers starting with a digit
	// (e.g. "export" -> "_export", "123code" -> "_123code"), matching upstream
	// openapi-generator's toSafeIdentifier. The original name is kept as the
	// property's baseName / JSON wire key by the caller, so serialization is
	// unaffected. Only true keywords are escaped — built-in type names like
	// "file" remain valid identifiers.
	return toSafeIdentifier(result)
}

// ToParamName converts a parameter name
func (g *BaseGenerator) ToParamName(name string) string {
	name = sanitizeIdentifierName(name)
	name = Camelize(name, true)
	return toSafeIdentifier(name)
}

// toSafeIdentifier escapes a camelized identifier that is a TypeScript reserved
// keyword or that starts with a digit, mirroring upstream openapi-generator's
// AbstractTypeScriptClientCodegen#toSafeIdentifier.
func toSafeIdentifier(name string) string {
	if tsReservedKeywords[strings.ToLower(name)] || (len(name) > 0 && unicode.IsDigit(rune(name[0]))) {
		return "_" + name
	}
	return name
}

// SanitizeOperationId sanitizes an operation ID
func (g *BaseGenerator) SanitizeOperationId(operationId string) string {
	return SanitizeName(operationId)
}

// toTypescriptTypeName converts a name to a valid TypeScript type name
func (g *BaseGenerator) toTypescriptTypeName(name, safePrefix string) string {
	// Sanitize name, but keep | and space for union types. Non-word characters
	// (e.g. the dots in a Kubernetes-style "io.k8s...NamedRuleWithOperations"
	// schema name) become a word boundary rather than being deleted, so Camelize
	// capitalizes each segment instead of collapsing them into one run — matching
	// upstream openapi-generator's sanitizeName(name, "(?![| ])\\W", ...), which
	// replaces the dot/colon/hyphen/etc. separators with "_" before camelizing.
	name = typeNameSeparatorPattern.ReplaceAllString(name, "_")
	name = Camelize(name, false)

	// Handle reserved words
	if g.IsReservedWord(name) {
		return safePrefix + name
	}

	// Handle names starting with a digit
	if len(name) > 0 && unicode.IsDigit(rune(name[0])) {
		return safePrefix + name
	}

	// Handle language primitives
	if g.IsPrimitive(name) {
		return safePrefix + name
	}

	return name
}

// ToModelFilename returns the model file name
func (g *BaseGenerator) ToModelFilename(name string) string {
	// name is already the classname (e.g., "Pet"), just return it
	return name
}

// ToApiFilename returns the API file name
func (g *BaseGenerator) ToApiFilename(name string) string {
	// name is already the API classname (e.g., "PetsApi"), just return it
	return name
}

// FromModel converts an OpenAPI schema to a CodegenModel
func (g *BaseGenerator) FromModel(name string, schema any) *codegen.CodegenModel {
	// This is a placeholder - actual implementation requires schema parsing
	cm := &codegen.CodegenModel{
		Name:          g.EscapeReservedWord(name),
		SchemaName:    name,
		Classname:     g.ToModelName(name),
		ClassVarName:  g.ToVarName(name),
		ClassFilename: g.ToModelFilename(name),
	}
	return cm
}

// FromOperation converts an OpenAPI operation to a CodegenOperation
func (g *BaseGenerator) FromOperation(path, httpMethod string, operation any) *codegen.CodegenOperation {
	// This is a placeholder - actual implementation requires operation parsing
	co := &codegen.CodegenOperation{
		Path:       path,
		HttpMethod: strings.ToUpper(httpMethod),
	}
	return co
}

// FromProperty converts an OpenAPI property to a CodegenProperty
func (g *BaseGenerator) FromProperty(name string, schema any, required bool) *codegen.CodegenProperty {
	cp := &codegen.CodegenProperty{
		Name:     g.ToVarName(name),
		BaseName: name,
		Required: required,
	}
	return cp
}

// FromParameter converts an OpenAPI parameter to a CodegenParameter
func (g *BaseGenerator) FromParameter(parameter any) *codegen.CodegenParameter {
	return &codegen.CodegenParameter{}
}

// FromResponse converts an OpenAPI response to a CodegenResponse
func (g *BaseGenerator) FromResponse(code string, response any) *codegen.CodegenResponse {
	return &codegen.CodegenResponse{
		Code: code,
	}
}

// FromSecurityScheme converts security scheme to CodegenSecurity
func (g *BaseGenerator) FromSecurityScheme(name string, scheme any) *codegen.CodegenSecurity {
	return &codegen.CodegenSecurity{
		Name: name,
	}
}

// PostProcessModels post-processes models
func (g *BaseGenerator) PostProcessModels(models []*codegen.CodegenModel) []*codegen.CodegenModel {
	return models
}

// PostProcessOperations post-processes operations
func (g *BaseGenerator) PostProcessOperations(operations []*codegen.CodegenOperation) []*codegen.CodegenOperation {
	return operations
}

// GetSupportingFiles returns supporting files
func (g *BaseGenerator) GetSupportingFiles() []generator.SupportingFile {
	return g.SupportingFiles
}

// GetApiTemplateFiles returns API template files
func (g *BaseGenerator) GetApiTemplateFiles() map[string]string {
	return g.ApiTemplateFiles
}

// GetModelTemplateFiles returns model template files
func (g *BaseGenerator) GetModelTemplateFiles() map[string]string {
	return g.ModelTemplateFiles
}

// GetConfig returns the generator config
func (g *BaseGenerator) GetConfig() *config.GeneratorConfig {
	return g.Config
}

// SetConfig sets the generator config
func (g *BaseGenerator) SetConfig(cfg *config.GeneratorConfig) {
	g.Config = cfg
}

// Helper functions

func copyMap(m map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range m {
		result[k] = v
	}
	return result
}

// copyMapBool creates a shallow copy of a boolean map.
func copyMapBool(m map[string]bool) map[string]bool {
	result := make(map[string]bool)
	for k, v := range m {
		result[k] = v
	}
	return result
}

// sanitizeSeparators replaces the separator characters upstream openapi-generator's
// DefaultCodegen#sanitizeName converts to "_" (nested bracket/paren notation, ".",
// ":", "-", "|", " ", "/", "\"). It is shared by every sanitizer below; what differs
// per caller — mirroring Java, where toApiName/sanitizeOperationId use the default
// sanitizeName(name, "\\W") while toVarName/toParamName use sanitizeName(name,
// "[^\\w$]") — is only the final catch-all pattern applied afterward.
func sanitizeSeparators(name string) string {
	name = strings.ReplaceAll(name, "[]", "")
	name = strings.ReplaceAll(name, "[", "_")
	name = strings.ReplaceAll(name, "]", "")
	name = strings.ReplaceAll(name, "(", "_")
	name = strings.ReplaceAll(name, ")", "")
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, "|", "_")
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, `\`, "_")
	return name
}

// SanitizeName sanitizes a name for use as a TypeScript type/API name or operation
// ID. It mirrors upstream openapi-generator's default DefaultCodegen#sanitizeName
// (removeCharRegEx "\\W", i.e. no "$" exception): a known set of separators
// (brackets, parens, ".", ":", "-", "|", " ", "/", "\") become "_", while any other
// punctuation — including "$" — is dropped rather than turned into a separator, so
// "input.name" becomes "input_name" but "a+b" becomes "ab" (not "a_b").
//
// Property and parameter names must NOT use this function — see
// sanitizeIdentifierName, which upstream reaches via a different removeCharRegEx
// ("[^\\w$]") that keeps "$" instead of stripping it.
func SanitizeName(name string) string {
	// Handle special cases for emoji reactions
	if name == "+1" {
		return "plus1"
	}
	if name == "-1" {
		return "minus1"
	}
	if name == "$" {
		return "value"
	}

	// Drop everything else that isn't a word character.
	return nonWordPattern.ReplaceAllString(sanitizeSeparators(name), "")
}

// sanitizeIdentifierName sanitizes a name for use as a TypeScript variable or
// parameter identifier, mirroring upstream openapi-generator's toVarName/toParamName
// (AbstractTypeScriptClientCodegen.java:585-612,566-582). It differs from SanitizeName
// in two ways, both taken from Java:
//
//  1. "$" is preserved rather than stripped (Java calls sanitizeName(name, "[^\\w$]"),
//     not the default "\\W"), so a property named "$ref" stays "$ref" instead of
//     becoming "ref".
//  2. a name that sanitizes down to a bare "_" is rewritten to "_u" before camelizing
//     (Java: `if ("_".equals(name)) { name = "_u"; }`), because Camelize treats a lone
//     "_" as pure separator and would otherwise collapse it to the empty string,
//     producing an uncompilable `?: string;` field with no identifier at all.
func sanitizeIdentifierName(name string) string {
	if name == "+1" {
		return "plus1"
	}
	if name == "-1" {
		return "minus1"
	}
	if name == "$" {
		return "value"
	}

	name = nonWordDollarPattern.ReplaceAllString(sanitizeSeparators(name), "")
	if name == "_" {
		name = "_u"
	}
	return name
}

// nonWordPattern matches any character that is not a word character (letters,
// digits, underscore), used as the final catch-all step of SanitizeName.
var nonWordPattern = regexp.MustCompile(`\W`)

// nonWordDollarPattern is nonWordPattern's counterpart for sanitizeIdentifierName:
// it additionally spares "$", matching Java's toVarName/toParamName removeCharRegEx.
var nonWordDollarPattern = regexp.MustCompile(`[^\w$]`)

// typeNameSeparatorPattern is toTypescriptTypeName's separator-to-underscore
// pattern (word chars, "|", and " " survive), hoisted so it compiles once instead
// of on every call — toTypescriptTypeName runs once per model, so ~900 times on a
// corpus the size of the Kubernetes spec.
var typeNameSeparatorPattern = regexp.MustCompile(`[^\w| ]`)

// Camelize converts a string to camelCase or PascalCase
func Camelize(s string, lowercaseFirst bool) string {
	if s == "" {
		return s
	}

	// Split on non-alphanumeric characters and camelCase boundaries
	var words []string
	var current strings.Builder

	for i, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '$' {
			// Non-alphanumeric character - end current word. "$" is kept out of
			// this branch (and so stays part of the word) because it is a valid
			// TypeScript identifier character upstream openapi-generator's own
			// camelize() never strips — e.g. "$ref" must camelize to "$ref", not
			// "ref". See sanitizeIdentifierName, which is what lets a leading "$"
			// reach here instead of being sanitized away first.
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
			continue
		}

		// Check for camelCase boundary (lowercase followed by uppercase)
		if i > 0 && r >= 'A' && r <= 'Z' && current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}

		current.WriteRune(r)
	}

	if current.Len() > 0 {
		words = append(words, current.String())
	}

	var result strings.Builder

	for i, word := range words {
		if word == "" {
			continue
		}
		if i == 0 && lowercaseFirst {
			result.WriteString(strings.ToLower(word[:1]) + word[1:])
		} else {
			result.WriteString(strings.ToUpper(word[:1]) + strings.ToLower(word[1:]))
		}
	}

	return result.String()
}

// Underscore converts a string to snake_case
func Underscore(s string) string {
	// Insert underscore before uppercase letters
	re := regexp.MustCompile(`([a-z])([A-Z])`)
	s = re.ReplaceAllString(s, "${1}_${2}")
	return strings.ToLower(s)
}

// Dashize converts a string to kebab-case
func Dashize(s string) string {
	return strings.ReplaceAll(Underscore(s), "_", "-")
}
