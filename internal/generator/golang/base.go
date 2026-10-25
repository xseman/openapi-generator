// Package golang provides the Go code generator; the name is golang because
// go is a keyword.
package golang

import (
	"fmt"
	"maps"
	"strings"
	"unicode"

	"github.com/xseman/openapi-generator/internal/codegen"
	"github.com/xseman/openapi-generator/internal/config"
	"github.com/xseman/openapi-generator/internal/generator"
)

// TypeMapping maps OpenAPI types to the TypeScript-shaped DataType strings
// the parser builds on (Array<X>, any, number). Go types are derived from a
// property's flags and tree instead, see GoType.
var TypeMapping = map[string]string{
	"array":   "Array",
	"boolean": "boolean",
	"integer": "number",
	"number":  "number",
	"object":  "any",
	"string":  "string",
	"AnyType": "any",
}

// RuntimeNames are the identifiers runtime.go declares; a model taking one
// is renamed with a Model suffix.
var RuntimeNames = map[string]bool{
	"BasePath": true, "Doer": true, "RequestEditorFn": true, "Configuration": true,
	"ResponseError": true, "Opt": true, "Some": true, "Null": true, "Ptr": true,
	"Date": true, "DateTime": true, "File": true,
}

// keywords are Go's reserved words, for the package name.
var keywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true,
	"interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
}

// initialisms are written in one case, as golint's commonInitialisms.
var initialisms = map[string]bool{
	"ACL": true, "API": true, "ASCII": true, "CPU": true, "CSS": true, "DNS": true,
	"EOF": true, "GUID": true, "HTML": true, "HTTP": true, "HTTPS": true, "ID": true,
	"IP": true, "JSON": true, "LHS": true, "QPS": true, "RAM": true, "RHS": true,
	"RPC": true, "SLA": true, "SMTP": true, "SQL": true, "SSH": true, "TCP": true,
	"TLS": true, "TTL": true, "UDP": true, "UI": true, "UID": true, "UUID": true,
	"URI": true, "URL": true, "UTF8": true, "VM": true, "XML": true, "XMPP": true,
	"XSRF": true, "XSS": true,
}

// buildSuffixes are the file name suffixes the go tool reads as a build
// constraint or a test file (model_test.go, api_linux.go).
var buildSuffixes = map[string]bool{
	"test": true, "aix": true, "android": true, "darwin": true, "dragonfly": true,
	"freebsd": true, "hurd": true, "illumos": true, "ios": true, "js": true,
	"linux": true, "nacl": true, "netbsd": true, "openbsd": true, "plan9": true,
	"solaris": true, "wasip1": true, "windows": true, "zos": true, "386": true,
	"amd64": true, "arm": true, "arm64": true, "loong64": true, "mips": true,
	"mips64": true, "mips64le": true, "mipsle": true, "ppc64": true, "ppc64le": true,
	"riscv64": true, "s390x": true, "wasm": true,
}

// BaseGenerator holds what a Go generator shares: the CodegenConfig plumbing
// and Go naming.
type BaseGenerator struct {
	Config *config.GeneratorConfig

	TypeMapping   map[string]string
	ReservedWords map[string]bool

	ModelPackage string
	ApiPackage   string

	ApiTemplateFiles     map[string]string
	ModelTemplateFiles   map[string]string
	SupportingFiles      []generator.SupportingFile
	AdditionalProperties map[string]any
}

// NewBaseGenerator creates a new Go base generator.
func NewBaseGenerator() *BaseGenerator {
	return &BaseGenerator{
		TypeMapping:          maps.Clone(TypeMapping),
		ReservedWords:        maps.Clone(RuntimeNames),
		ApiTemplateFiles:     make(map[string]string),
		ModelTemplateFiles:   make(map[string]string),
		AdditionalProperties: make(map[string]any),
	}
}

// GetTypeMapping returns the type mapping.
func (g *BaseGenerator) GetTypeMapping() map[string]string { return g.TypeMapping }

// GetReservedWords returns the reserved words.
func (g *BaseGenerator) GetReservedWords() map[string]bool { return g.ReservedWords }

// GetLanguageSpecificPrimitives returns the primitive types.
func (g *BaseGenerator) GetLanguageSpecificPrimitives() map[string]bool { return nil }

// GetImportMapping returns the import mapping.
func (g *BaseGenerator) GetImportMapping() map[string]string { return nil }

// GetAdditionalProperties returns additional properties for templates.
func (g *BaseGenerator) GetAdditionalProperties() map[string]any { return g.AdditionalProperties }

// IsReservedWord reports whether word is a name runtime.go declares.
func (g *BaseGenerator) IsReservedWord(word string) bool { return g.ReservedWords[word] }

// EscapeReservedWord suffixes a reserved name with Model.
func (g *BaseGenerator) EscapeReservedWord(name string) string {
	if g.IsReservedWord(name) {
		return name + "Model"
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

// ToModelName converts a schema name to an exported Go type name.
func (g *BaseGenerator) ToModelName(name string) string {
	return g.EscapeReservedWord(exported(name, "Model"))
}

// ToApiName converts a tag to the API type name, PetAPI.
func (g *BaseGenerator) ToApiName(name string) string {
	return exported(name, "Default") + "API"
}

// ToVarName converts a property or parameter name to an exported field name.
func (g *BaseGenerator) ToVarName(name string) string { return exported(name, "Var") }

// ToParamName converts a parameter name to an exported field name.
func (g *BaseGenerator) ToParamName(name string) string { return g.ToVarName(name) }

// SanitizeOperationId converts an operation ID to an exported method name.
func (g *BaseGenerator) SanitizeOperationId(operationId string) string {
	return exported(operationId, "Op")
}

// ToModelFilename returns the file name of a model, model_pet.
func (g *BaseGenerator) ToModelFilename(name string) string {
	return fileName("model_" + Snake(name))
}

// ToApiFilename returns the file name of an API type, api_pet for PetAPI.
func (g *BaseGenerator) ToApiFilename(name string) string {
	return fileName("api_" + Snake(strings.TrimSuffix(name, "API")))
}

// FromModel is unused: the parser builds models.
func (g *BaseGenerator) FromModel(name string, _ any) *codegen.CodegenModel {
	return &codegen.CodegenModel{Name: name, Classname: g.ToModelName(name)}
}

// FromOperation is unused: the parser builds operations.
func (g *BaseGenerator) FromOperation(path, httpMethod string, _ any) *codegen.CodegenOperation {
	return &codegen.CodegenOperation{Path: path, HttpMethod: strings.ToUpper(httpMethod)}
}

// FromProperty is unused: the parser builds properties.
func (g *BaseGenerator) FromProperty(name string, _ any, required bool) *codegen.CodegenProperty {
	return &codegen.CodegenProperty{Name: g.ToVarName(name), BaseName: name, Required: required}
}

// FromParameter is unused: the parser builds parameters.
func (g *BaseGenerator) FromParameter(_ any) *codegen.CodegenParameter {
	return &codegen.CodegenParameter{}
}

// FromResponse is unused: the parser builds responses.
func (g *BaseGenerator) FromResponse(code string, _ any) *codegen.CodegenResponse {
	return &codegen.CodegenResponse{Code: code}
}

// FromSecurityScheme is unused: the parser builds security schemes.
func (g *BaseGenerator) FromSecurityScheme(name string, _ any) *codegen.CodegenSecurity {
	return &codegen.CodegenSecurity{Name: name}
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

// GoType is the Go type of a property, from its flags and its Items tree.
// named is the type declared for an inline enum or object at its leaf, or
// the enum or union model a $ref names; "" types an enum by its values, an
// object as a map and a union as raw JSON.
func GoType(p *codegen.CodegenProperty, named string) string {
	switch {
	case p == nil:
		return "any"
	case p.IsArray:
		return "[]" + itemType(p.Items, named)
	case p.IsMap:
		return "map[string]" + itemType(p.Items, named)
	case named != "" && (p.Ref != "" || p.IsEnum || len(p.Vars) > 0 || p.ComposedSchemas != nil):
		return named
	case isComposite(p.DataType):
		// ponytail: an intersection, or a union not named (a union model's
		// own inline member), stays raw JSON; the upgrade is a named type.
		return "json.RawMessage"
	case p.IsEnum:
		return EnumBaseType(p.AllowableValues, p.IsBoolean)
	case p.IsModel:
		return p.DataType
	}

	return scalarType(p.IsString, p.IsDate, p.IsDateTime, p.IsBinary || p.IsByteArray, p.IsInteger, p.IsLong,
		p.IsNumber, p.IsFloat, p.IsBoolean, p.OpenApiType == "object")
}

// itemType is the Go type of a slice element or map value: GoType, a
// pointer when the item is nullable and not nil-able already.
func itemType(p *codegen.CodegenProperty, named string) string {
	t := GoType(p, named)
	if p != nil && p.IsNullable && !isContainer(t) {
		return "*" + t
	}

	return t
}

// GoParamType is the Go type of a parameter; a file is a File. named is
// the type declared for an inline object, as for GoType.
func GoParamType(p *codegen.CodegenParameter, named string) string {
	switch {
	case p.IsArray && p.IsFile:
		return "[]File"
	case p.IsArray:
		return "[]" + itemType(p.Items, named)
	case (len(p.Vars) > 0 || p.ComposedSchemas != nil || p.Ref != "") && named != "":
		return named
	case p.IsMap:
		return "map[string]" + itemType(p.Items, named)
	case isComposite(p.DataType):
		return "json.RawMessage"
	case p.IsFile || p.IsBinary:
		return "File"
	case p.IsEnum:
		return EnumBaseType(p.AllowableValues, p.IsBoolean)
	case p.IsModel:
		return p.DataType
	}

	return scalarType(p.IsString, p.IsDate, p.IsDateTime, p.IsByteArray, p.IsInteger, p.IsLong,
		p.IsNumber, p.IsFloat, p.IsBoolean, false)
}

func scalarType(isString, isDate, isDateTime, isBytes, isInteger, isLong, isNumber, isFloat, isBoolean, isObject bool) string {
	switch {
	case isDate:
		return "Date"
	case isDateTime:
		return "DateTime"
	case isBytes:
		return "[]byte"
	case isString:
		return "string"
	case isLong:
		return "int64"
	case isInteger:
		return "int"
	case isFloat:
		return "float32"
	case isNumber:
		return "float64"
	case isBoolean:
		return "bool"
	case isObject:
		return "map[string]any"
	}

	return "any"
}

// EnumBaseType is the Go type an enum's values have: string when they are
// strings (isString on the enumVars), else bool, int when every value is
// whole, float64 otherwise; any when they are of more than one JSON type
// (pydantic's Literal["auto", 0]), which no Go constant type holds.
func EnumBaseType(allowable map[string]any, isBoolean bool) string {
	values, _ := allowable["values"].([]any)

	kinds := map[string]bool{}
	for _, v := range values {
		kinds[fmt.Sprintf("%T", v)] = true
	}

	if kinds["string"] && len(kinds) > 1 {
		return "any"
	}

	vars, _ := allowable["enumVars"].([]map[string]any)
	if len(vars) > 0 && vars[0]["isString"] == true {
		return "string"
	}

	if isBoolean {
		return "bool"
	}

	for _, v := range values {
		if f, ok := v.(float64); ok && f != float64(int64(f)) {
			return "float64"
		}
	}

	return "int"
}

// isComposite reports whether the parser typed a value as a union or
// intersection of members ("A | B", "A & B").
func isComposite(dataType string) bool {
	return strings.Contains(dataType, " | ") || strings.Contains(dataType, " & ")
}

// exported converts s to an exported Go identifier: PascalCase words with
// initialisms in upper case, prefix in front when it would not start with
// an upper-case letter (a digit, an empty name).
func exported(s, prefix string) string {
	var b strings.Builder

	for _, w := range splitWords(s) {
		if up := strings.ToUpper(w); initialisms[up] {
			b.WriteString(up)
			continue
		}

		r := []rune(strings.ToLower(w))
		r[0] = unicode.ToUpper(r[0])
		b.WriteString(string(r))
	}

	name := b.String()
	if r := []rune(name); len(r) == 0 || !unicode.IsUpper(r[0]) {
		name = prefix + name
	}

	return name
}

// Snake converts s to snake_case, keeping an initialism one word.
func Snake(s string) string {
	words := splitWords(s)
	for i, w := range words {
		words[i] = strings.ToLower(w)
	}

	return strings.Join(words, "_")
}

// fileName suffixes a name the go tool would read as a build constraint
// or a test file with an underscore.
func fileName(name string) string {
	if i := strings.LastIndexByte(name, '_'); i >= 0 && buildSuffixes[name[i+1:]] {
		return name + "_"
	}

	return name
}

// splitWords splits s on what is neither letter nor digit, before an upper
// case letter that follows a lower-case one or a digit, and before the last
// letter of an upper-case run that a lower-case letter follows (URLPath ->
// URL, Path).
func splitWords(s string) []string {
	var (
		words []string
		cur   []rune
	)

	flush := func() {
		if len(cur) > 0 {
			words = append(words, string(cur))
			cur = nil
		}
	}

	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			flush()
			continue
		}

		if n := len(cur); n > 0 {
			prev := cur[n-1]

			switch {
			case unicode.IsUpper(r) && (unicode.IsLower(prev) || unicode.IsDigit(prev)):
				flush()
			case unicode.IsLower(r) && unicode.IsUpper(prev) && n > 1 && unicode.IsUpper(cur[n-2]):
				words = append(words, string(cur[:n-1]))
				cur = []rune{prev}
			}
		}

		cur = append(cur, r)
	}

	flush()

	return words
}
