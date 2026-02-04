package typescript

import (
	"strings"

	"github.com/xseman/openapi-generator/internal/codegen"
	"github.com/xseman/openapi-generator/internal/config"
	"github.com/xseman/openapi-generator/internal/generator"
)

// FetchGenerator implements the typescript-fetch generator.
// It mirrors TypeScriptFetchClientCodegen in Java.
type FetchGenerator struct {
	*BaseGenerator

	// Fetch-specific configuration
	WithPackageJson           bool
	ImportFileExtension       string
	UseSingleRequestParameter bool
	PrefixParameterInterfaces bool
	WithoutRuntimeChecks      bool
	StringEnums               bool
	FileNaming                string
	WithInterfaces            bool
	ValidationAttributes      bool
}

// NewFetchGenerator creates a new TypeScript Fetch generator
func NewFetchGenerator() *FetchGenerator {
	base := NewBaseGenerator()

	g := &FetchGenerator{
		BaseGenerator:             base,
		UseSingleRequestParameter: true,
		FileNaming:                "camelCase",
	}

	// Set up template files
	g.ApiTemplateFiles["apis.mustache"] = ".ts"

	// Add extra reserved words
	g.addExtraReservedWords()

	return g
}

// GetName returns the generator name
func (g *FetchGenerator) GetName() string {
	return "typescript-fetch"
}

// GetTag returns the generator type
func (g *FetchGenerator) GetTag() generator.GeneratorType {
	return generator.GeneratorTypeClient
}

// GetHelp returns the help text
func (g *FetchGenerator) GetHelp() string {
	return "Generates a TypeScript client library using Fetch API."
}

// ProcessOpts processes CLI options and initializes the generator
func (g *FetchGenerator) ProcessOpts() error {
	// Process TypeScript config
	if g.TSConfig != nil {
		g.WithPackageJson = g.TSConfig.WithPackageJson
		g.ImportFileExtension = g.TSConfig.ImportFileExtension
		g.UseSingleRequestParameter = g.TSConfig.UseSingleRequestParameter
		g.PrefixParameterInterfaces = g.TSConfig.PrefixParameterInterfaces
		g.WithoutRuntimeChecks = g.TSConfig.WithoutRuntimeChecks
		g.StringEnums = g.TSConfig.StringEnums
		g.WithInterfaces = g.TSConfig.WithInterfaces
		g.ValidationAttributes = g.TSConfig.GenerateValidationAttributes

		if g.TSConfig.FileNaming != "" {
			g.FileNaming = g.TSConfig.FileNaming
		}
		if g.TSConfig.ModelPropertyNaming != "" {
			g.ModelPropertyNaming = config.ModelPropertyNamingType(g.TSConfig.ModelPropertyNaming)
		}
	}

	// Set up source directory
	g.SourceDir = ""
	g.ApiPackage = "apis"
	g.ModelPackage = "models"

	// Set up additional properties for templates
	g.AdditionalProperties["withPackageJson"] = g.WithPackageJson
	g.AdditionalProperties["importFileExtension"] = g.ImportFileExtension
	g.AdditionalProperties["useSingleRequestParameter"] = g.UseSingleRequestParameter
	g.AdditionalProperties["prefixParameterInterfaces"] = g.PrefixParameterInterfaces
	g.AdditionalProperties["withoutRuntimeChecks"] = g.WithoutRuntimeChecks
	g.AdditionalProperties["stringEnums"] = g.StringEnums
	g.AdditionalProperties["withInterfaces"] = g.WithInterfaces
	g.AdditionalProperties["validationAttributes"] = g.ValidationAttributes
	g.AdditionalProperties["isOriginalModelPropertyNaming"] = g.ModelPropertyNaming == config.PropertyNamingOriginal
	g.AdditionalProperties["modelPropertyNaming"] = string(g.ModelPropertyNaming)

	// Add supporting files
	g.SupportingFiles = append(g.SupportingFiles,
		generator.NewSupportingFile("index.mustache", "", "index.ts"),
		generator.NewSupportingFile("runtime.mustache", "", "runtime.ts"),
	)

	// Add package files if requested
	if g.WithPackageJson {
		g.SupportingFiles = append(g.SupportingFiles,
			generator.NewSupportingFile("package.mustache", "", "package.json"),
			generator.NewSupportingFile("tsconfig.mustache", "", "tsconfig.json"),
			generator.NewSupportingFile("README.mustache", "", "README.md"),
		)
	}

	// Add model template
	// Models are always generated, but the template content changes based on withoutRuntimeChecks
	g.ModelTemplateFiles["models.mustache"] = ".ts"

	// Update type mapping for Date types when runtime checks are enabled
	if !g.WithoutRuntimeChecks {
		g.TypeMapping["date"] = "Date"
		g.TypeMapping["DateTime"] = "Date"
		g.TypeMapping["date-time"] = "Date"
	}

	return nil
}

// ToApiFilename returns the API file name with file naming convention applied
func (g *FetchGenerator) ToApiFilename(name string) string {
	return g.convertUsingFileNamingConvention(g.BaseGenerator.ToApiFilename(name))
}

// ToModelFilename returns the model file name with file naming convention applied
func (g *FetchGenerator) ToModelFilename(name string) string {
	return g.convertUsingFileNamingConvention(g.BaseGenerator.ToModelFilename(name))
}

// GetTypeDeclaration returns the type declaration for file/binary types
func (g *FetchGenerator) GetTypeDeclaration(schemaType, format string) string {
	if schemaType == "file" || format == "binary" {
		return "Blob"
	}
	return g.BaseGenerator.GetTypeDeclaration(schemaType, format)
}

// EscapeReservedWord escapes reserved words
func (g *FetchGenerator) EscapeReservedWord(name string) string {
	return g.BaseGenerator.EscapeReservedWord(name)
}

// PostProcessModels post-processes models for TypeScript-Fetch
func (g *FetchGenerator) PostProcessModels(models []*codegen.CodegenModel) []*codegen.CodegenModel {
	for _, cm := range models {
		g.processCodeGenModel(cm)
	}
	return models
}

// PostProcessOperations post-processes operations for TypeScript-Fetch
func (g *FetchGenerator) PostProcessOperations(operations []*codegen.CodegenOperation) []*codegen.CodegenOperation {
	for _, op := range operations {
		g.escapeOperationId(op)
		g.updateOperationParameterForEnum(op)
		g.addOperationObjectResponseInformation(op)
	}
	return operations
}

// processCodeGenModel processes a model for TypeScript-Fetch specific transformations
func (g *FetchGenerator) processCodeGenModel(cm *codegen.CodegenModel) {
	// Process enum names
	for _, v := range cm.Vars {
		g.processCodegenProperty(v, cm.Classname)
	}

	// Process parent vars for inheritance
	if cm.Parent != "" {
		for _, v := range cm.AllVars {
			if v.IsEnum {
				v.DatatypeWithEnum = strings.Replace(
					v.DatatypeWithEnum,
					v.EnumName,
					cm.Classname+v.EnumName,
					1,
				)
			}
		}
	}
}

// processCodegenProperty processes a property for TypeScript-Fetch specific transformations
func (g *FetchGenerator) processCodegenProperty(prop *codegen.CodegenProperty, parentClassName string) {
	// Name enum with model name, e.g., StatusEnum => PetStatusEnum
	if prop.IsEnum {
		prop.DatatypeWithEnum = strings.Replace(
			prop.DatatypeWithEnum,
			prop.EnumName,
			parentClassName+prop.EnumName,
			1,
		)

		// Update default value
		if prop.DefaultValue != "" && prop.DefaultValue != "undefined" {
			if idx := strings.Index(prop.DefaultValue, "."); idx != -1 {
				prop.DefaultValue = prop.DatatypeWithEnum + prop.DefaultValue[idx:]
			}
		}
	}
}

// escapeOperationId escapes operation IDs that conflict with imports or naming patterns
func (g *FetchGenerator) escapeOperationId(op *codegen.CodegenOperation) {
	// Check for conflict with "Request" suffix import
	param := op.OperationIdCamelCase + "Request"
	for _, imp := range op.Imports {
		if imp == param {
			op.OperationIdCamelCase += "Operation"
			op.OperationIdLowerCase += "operation"
			op.OperationIdSnakeCase += "_operation"
			op.Nickname = op.OperationIdCamelCase
			break
		}
	}

	// Check if operationId ends with "Raw" which would conflict with the internal method naming pattern
	// The template generates methodNameRaw() for internal methods, so an operationId like "fooRaw"
	// would conflict with the internal method of operation "foo"
	if strings.HasSuffix(op.OperationIdCamelCase, "Raw") {
		op.OperationIdCamelCase += "Method"
		op.OperationIdLowerCase += "method"
		op.OperationIdSnakeCase += "_method"
		op.Nickname = op.OperationIdCamelCase
	}
}

// updateOperationParameterForEnum updates parameter enum names
func (g *FetchGenerator) updateOperationParameterForEnum(op *codegen.CodegenOperation) {
	for _, param := range op.AllParams {
		if param.IsEnum {
			param.DatatypeWithEnum = strings.Replace(
				param.DatatypeWithEnum,
				param.EnumName,
				op.OperationIdCamelCase+param.EnumName,
				1,
			)
		}
	}
}

// addOperationObjectResponseInformation handles object response types
func (g *FetchGenerator) addOperationObjectResponseInformation(op *codegen.CodegenOperation) {
	if op.ReturnType == "object" {
		op.IsMap = true
		op.ReturnSimpleType = false
	}
}

// convertUsingFileNamingConvention applies file naming convention
func (g *FetchGenerator) convertUsingFileNamingConvention(name string) string {
	switch g.FileNaming {
	case "kebab-case":
		return Dashize(Underscore(name))
	case "camelCase":
		return Camelize(name, true)
	default: // PascalCase
		return name
	}
}

// addExtraReservedWords adds typescript-fetch specific reserved words
func (g *FetchGenerator) addExtraReservedWords() {
	extraWords := []string{
		"BASE_PATH", "BaseAPI", "RequiredError", "COLLECTION_FORMATS",
		"FetchAPI", "ConfigurationParameters", "Configuration", "configuration",
		"HTTPMethod", "HTTPHeaders", "HTTPQuery", "HTTPBody",
		"ModelPropertyNaming", "FetchParams", "RequestOpts", "exists",
		"RequestContext", "ResponseContext", "Middleware", "ApiResponse",
		"ResponseTransformer", "JSONApiResponse", "VoidApiResponse",
		"BlobApiResponse", "TextApiResponse", "Index",
	}
	for _, word := range extraWords {
		g.ReservedWords[word] = true
	}
}
