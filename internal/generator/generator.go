// Package generator provides the base interfaces and types for code generators.
package generator

import (
	"github.com/xseman/openapi-generator/internal/codegen"
	"github.com/xseman/openapi-generator/internal/config"
)

// Type aliases for codegen types to be used by CLI and other packages
type (
	CodegenModel     = codegen.CodegenModel
	CodegenOperation = codegen.CodegenOperation
	CodegenProperty  = codegen.CodegenProperty
	CodegenParameter = codegen.CodegenParameter
	CodegenResponse  = codegen.CodegenResponse
	CodegenSecurity  = codegen.CodegenSecurity
)

// CodegenConfig is the interface that all generators must implement.
// This mirrors the Java CodegenConfig interface.
type CodegenConfig interface {
	// GetName returns the generator name (e.g., "typescript-fetch")
	GetName() string

	// GetTag returns the generator type
	GetTag() GeneratorType

	// GetHelp returns the help text for the generator
	GetHelp() string

	// ProcessOpts processes CLI options and initializes the generator
	ProcessOpts() error

	// ToModelName converts a schema name to a model class name
	ToModelName(name string) string

	// ToApiName converts a tag to an API class name
	ToApiName(name string) string

	// ToVarName converts a property name to a variable name
	ToVarName(name string) string

	// ToParamName converts a parameter name
	ToParamName(name string) string

	// SanitizeOperationId sanitizes an operation ID
	SanitizeOperationId(operationId string) string

	// ToModelFilename returns the model file name
	ToModelFilename(name string) string

	// ToApiFilename returns the API file name
	ToApiFilename(name string) string

	// GetTypeDeclaration gets the language-specific type declaration
	GetTypeDeclaration(schemaType string, format string) string

	// GetSchemaType gets the language-specific type for a schema
	GetSchemaType(schemaType string, format string) string

	// IsReservedWord checks if a word is reserved
	IsReservedWord(word string) bool

	// EscapeReservedWord escapes a reserved word
	EscapeReservedWord(name string) string

	// FromModel converts an OpenAPI schema to a CodegenModel
	FromModel(name string, schema any) *codegen.CodegenModel

	// FromOperation converts an OpenAPI operation to a CodegenOperation
	FromOperation(path, httpMethod string, operation any) *codegen.CodegenOperation

	// FromProperty converts an OpenAPI property to a CodegenProperty
	FromProperty(name string, schema any, required bool) *codegen.CodegenProperty

	// FromParameter converts an OpenAPI parameter to a CodegenParameter
	FromParameter(parameter any) *codegen.CodegenParameter

	// FromResponse converts an OpenAPI response to a CodegenResponse
	FromResponse(code string, response any) *codegen.CodegenResponse

	// FromSecurityScheme converts an OpenAPI security scheme to CodegenSecurity
	FromSecurityScheme(name string, securityScheme any) *codegen.CodegenSecurity

	// PostProcessModels post-processes all models
	PostProcessModels(models []*codegen.CodegenModel) []*codegen.CodegenModel

	// PostProcessOperations post-processes operations
	PostProcessOperations(operations []*codegen.CodegenOperation) []*codegen.CodegenOperation

	// GetSupportingFiles returns the list of supporting files to generate
	GetSupportingFiles() []SupportingFile

	// GetApiTemplateFiles returns the API template files
	GetApiTemplateFiles() map[string]string

	// GetModelTemplateFiles returns the model template files
	GetModelTemplateFiles() map[string]string

	// GetConfig returns the generator configuration
	GetConfig() *config.GeneratorConfig

	// SetConfig sets the generator configuration
	SetConfig(cfg *config.GeneratorConfig)

	// GetAdditionalProperties returns the template additional properties
	GetAdditionalProperties() map[string]any

	// GetTypeMapping returns the type mapping
	GetTypeMapping() map[string]string

	// GetReservedWords returns the set of reserved words
	GetReservedWords() map[string]bool

	// GetLanguageSpecificPrimitives returns the set of primitive types
	GetLanguageSpecificPrimitives() map[string]bool

	// GetImportMapping returns the import mapping
	GetImportMapping() map[string]string
}

// GeneratorType represents the type of generator
type GeneratorType string

const (
	GeneratorTypeClient        GeneratorType = "CLIENT"
	GeneratorTypeServer        GeneratorType = "SERVER"
	GeneratorTypeDocumentation GeneratorType = "DOCUMENTATION"
	GeneratorTypeConfig        GeneratorType = "CONFIG"
	GeneratorTypeSchema        GeneratorType = "SCHEMA"
	GeneratorTypeOther         GeneratorType = "OTHER"
)

// SupportingFile represents a file to be generated
type SupportingFile struct {
	TemplateFile        string // Template file name
	Folder              string // Output folder relative to output directory
	DestinationFilename string // Output file name
}

// NewSupportingFile creates a new SupportingFile
func NewSupportingFile(templateFile, folder, destinationFilename string) SupportingFile {
	return SupportingFile{
		TemplateFile:        templateFile,
		Folder:              folder,
		DestinationFilename: destinationFilename,
	}
}

// TemplateData holds data passed to templates
type TemplateData struct {
	// Package info
	PackageName    string `json:"packageName"`
	ApiPackage     string `json:"apiPackage"`
	ModelPackage   string `json:"modelPackage"`
	InvokerPackage string `json:"invokerPackage"`

	// OpenAPI info
	AppName        string `json:"appName"`
	AppDescription string `json:"appDescription"`
	InfoEmail      string `json:"infoEmail"`
	InfoUrl        string `json:"infoUrl"`
	LicenseName    string `json:"licenseName"`
	LicenseUrl     string `json:"licenseUrl"`
	Version        string `json:"version"`
	BasePath       string `json:"basePath"`
	Host           string `json:"host"`

	// Generation info
	GeneratorClass   string `json:"generatorClass"`
	GeneratorVersion string `json:"generatorVersion"`
	GeneratedDate    string `json:"generatedDate"`

	// Models and APIs
	Models     []*codegen.CodegenModel     `json:"models"`
	Operations []*codegen.CodegenOperation `json:"operations"`
	ApiInfo    *ApiInfo                    `json:"apiInfo"`

	// Security
	AuthMethods []*codegen.CodegenSecurity `json:"authMethods"`

	// Servers
	Servers []map[string]any `json:"servers"`

	// Additional properties (generator-specific)
	AdditionalProperties map[string]any `json:"-"`
}

// ApiInfo holds information about API classes
type ApiInfo struct {
	Apis []*ApiClass `json:"apis"`
}

// ApiClass holds information about a single API class
type ApiClass struct {
	Classname     string              `json:"classname"`
	ClassVarName  string              `json:"classVarName"`
	ClassFilename string              `json:"classFilename"`
	BaseName      string              `json:"baseName"`
	Operations    *OperationGroup     `json:"operations"`
	Imports       []map[string]string `json:"imports"`
	HasImports    bool                `json:"hasImports"`
	Description   string              `json:"description"`
}

// OperationGroup holds a group of operations
type OperationGroup struct {
	Operation  []*codegen.CodegenOperation `json:"operation"`
	Classname  string                      `json:"classname"`
	PathPrefix string                      `json:"pathPrefix"`
}

// ModelData holds data for model template
type ModelData struct {
	Model      *codegen.CodegenModel `json:"model"`
	Models     []*ModelMap           `json:"models"`
	Imports    []map[string]string   `json:"imports"`
	HasImports bool                  `json:"hasImports"`
	TsImports  []map[string]string   `json:"tsImports"`
}

// ModelMap wraps a model for template rendering
type ModelMap struct {
	Model      *codegen.CodegenModel `json:"model"`
	ImportPath string                `json:"importPath"`
	HasImports bool                  `json:"hasImports"`
	TsImports  []map[string]string   `json:"tsImports"`
}
