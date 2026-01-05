// Package config provides configuration structures for code generators.
package config

// GeneratorConfig holds configuration for code generation.
type GeneratorConfig struct {
	// Input/Output
	InputSpec   string `json:"inputSpec"`
	OutputDir   string `json:"outputDir"`
	TemplateDir string `json:"templateDir,omitempty"`

	// Generator identification
	GeneratorName string `json:"generatorName"`

	// Package configuration
	PackageName    string `json:"packageName,omitempty"`
	ApiPackage     string `json:"apiPackage,omitempty"`
	ModelPackage   string `json:"modelPackage,omitempty"`
	InvokerPackage string `json:"invokerPackage,omitempty"`

	// Naming
	ModelNamePrefix string `json:"modelNamePrefix,omitempty"`
	ModelNameSuffix string `json:"modelNameSuffix,omitempty"`
	ApiNamePrefix   string `json:"apiNamePrefix,omitempty"`
	ApiNameSuffix   string `json:"apiNameSuffix,omitempty"`

	// Global flags
	SkipOverwrite       bool `json:"skipOverwrite,omitempty"`
	SkipValidateSpec    bool `json:"skipValidateSpec,omitempty"`
	StrictSpec          bool `json:"strictSpec,omitempty"`
	EnableMinimalUpdate bool `json:"enableMinimalUpdate,omitempty"`

	// Additional properties (generator-specific)
	AdditionalProperties map[string]any `json:"additionalProperties,omitempty"`

	// Global properties
	GlobalProperties map[string]any `json:"globalProperties,omitempty"`
}

// TypeScriptFetchConfig holds configuration specific to typescript-fetch generator.
type TypeScriptFetchConfig struct {
	// Package generation
	WithPackageJson bool `json:"withPackageJson,omitempty"`

	// Generation options
	WithInterfaces            bool   `json:"withInterfaces,omitempty"`
	UseSingleRequestParameter bool   `json:"useSingleRequestParameter"`
	PrefixParameterInterfaces bool   `json:"prefixParameterInterfaces,omitempty"`
	WithoutRuntimeChecks      bool   `json:"withoutRuntimeChecks,omitempty"`
	StringEnums               bool   `json:"stringEnums,omitempty"`
	ImportFileExtension       string `json:"importFileExtension,omitempty"`
	FileNaming                string `json:"fileNaming,omitempty"` // PascalCase, camelCase, kebab-case

	// Validation
	GenerateValidationAttributes bool `json:"validationAttributes,omitempty"`

	// Square brackets in array names
	UseSquareBracketsInArrayNames bool `json:"useSquareBracketsInArrayNames,omitempty"`

	// Model property naming
	ModelPropertyNaming string `json:"modelPropertyNaming,omitempty"` // original, camelCase, PascalCase, snake_case

	// Enum property naming
	EnumPropertyNaming string `json:"enumPropertyNaming,omitempty"`

	// Null-safe additional props
	NullSafeAdditionalProps bool `json:"nullSafeAdditionalProps,omitempty"`

	// Allow unicode identifiers
	AllowUnicodeIdentifiers bool `json:"allowUnicodeIdentifiers,omitempty"`

	// Prepend form or body parameters to the list
	PrependFormOrBodyParameters bool `json:"prependFormOrBodyParameters,omitempty"`

	// Sort params by required flag
	SortParamsByRequiredFlag bool `json:"sortParamsByRequiredFlag,omitempty"`

	// Sort model properties by required flag
	SortModelPropertiesByRequiredFlag bool `json:"sortModelPropertiesByRequiredFlag,omitempty"`

	// Ensure unique params
	EnsureUniqueParams bool `json:"ensureUniqueParams,omitempty"`

	// Legacy discriminator behavior
	LegacyDiscriminatorBehavior bool `json:"legacyDiscriminatorBehavior,omitempty"`

	// Disallow additional properties if not present
	DisallowAdditionalPropertiesIfNotPresent bool `json:"disallowAdditionalPropertiesIfNotPresent,omitempty"`

	// License
	LicenseName string `json:"licenseName,omitempty"`
	LicenseUrl  string `json:"licenseUrl,omitempty"`
}

// NewTypeScriptFetchConfig creates a new TypeScriptFetchConfig with default values.
func NewTypeScriptFetchConfig() *TypeScriptFetchConfig {
	return &TypeScriptFetchConfig{
		UseSingleRequestParameter: true,
		FileNaming:                "camelCase",
		ModelPropertyNaming:       "camelCase",
		SortParamsByRequiredFlag:  true,
		EnsureUniqueParams:        true,
	}
}

// FileNamingType represents the file naming convention
type FileNamingType string

const (
	FileNamingPascalCase FileNamingType = "PascalCase"
	FileNamingCamelCase  FileNamingType = "camelCase"
	FileNamingKebabCase  FileNamingType = "kebab-case"
)

// ModelPropertyNamingType represents the property naming convention
type ModelPropertyNamingType string

const (
	PropertyNamingOriginal   ModelPropertyNamingType = "original"
	PropertyNamingCamelCase  ModelPropertyNamingType = "camelCase"
	PropertyNamingPascalCase ModelPropertyNamingType = "PascalCase"
	PropertyNamingSnakeCase  ModelPropertyNamingType = "snake_case"
)
