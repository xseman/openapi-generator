// Package codegen provides data structures for code generation from OpenAPI specs.
// These structures mirror the Java implementation in openapi-generator.
package codegen

// CodegenModel represents a schema/model in the OpenAPI document.
// It is the Go equivalent of org.openapitools.codegen.CodegenModel.
type CodegenModel struct {
	// Identity
	Name          string `json:"name"`          // Name (escaped if reserved)
	SchemaName    string `json:"schemaName"`    // Original schema name from OAS
	Classname     string `json:"classname"`     // Language-specific class name
	ClassVarName  string `json:"classVarName"`  // Variable name for the class
	ClassFilename string `json:"classFilename"` // File name for the model

	// Documentation
	Title                string `json:"title"`
	Description          string `json:"description"`
	UnescapedDescription string `json:"unescapedDescription"`

	// Type info
	DataType       string `json:"dataType"` // Language-specific data type
	ArrayModelType string `json:"arrayModelType"`
	IsAlias        bool   `json:"isAlias"` // Is this an alias of a simple type?
	IsEnum         bool   `json:"isEnum"`
	IsArray        bool   `json:"isArray"`
	IsMap          bool   `json:"isMap"`
	IsNullable     bool   `json:"isNullable"`
	IsDeprecated   bool   `json:"isDeprecated"`

	// Primitive type flags
	IsString         bool `json:"isString"`
	IsInteger        bool `json:"isInteger"`
	IsLong           bool `json:"isLong"`
	IsNumber         bool `json:"isNumber"`
	IsNumeric        bool `json:"isNumeric"`
	IsFloat          bool `json:"isFloat"`
	IsDouble         bool `json:"isDouble"`
	IsDecimal        bool `json:"isDecimal"`
	IsDate           bool `json:"isDate"`
	IsDateTime       bool `json:"isDateTime"`
	IsBoolean        bool `json:"isBoolean"`
	IsFreeFormObject bool `json:"isFreeFormObject"`
	IsPrimitiveType  bool `json:"isPrimitiveType"`

	// Inheritance
	Parent       string          `json:"parent"` // Parent model name
	ParentSchema string          `json:"parentSchema"`
	Interfaces   []string        `json:"interfaces"` // Implemented interfaces
	AllParents   []string        `json:"allParents"` // All parent names
	ParentModel  *CodegenModel   `json:"-"`          // Reference to parent model
	Children     []*CodegenModel `json:"-"`          // Child models

	// Composition (oneOf/anyOf/allOf)
	OneOf           []string                `json:"oneOf"`       // Set of oneOf schema names
	OneOfModels     []string                `json:"oneOfModels"` // Non-primitive oneOf members for imports
	AnyOf           []string                `json:"anyOf"`       // Set of anyOf schema names
	AllOf           []string                `json:"allOf"`       // Set of allOf schema names
	ComposedSchemas *CodegenComposedSchemas `json:"composedSchemas"`

	// Properties
	Vars            []*CodegenProperty `json:"vars"`         // Properties (without parent's)
	AllVars         []*CodegenProperty `json:"allVars"`      // All properties (with parent's)
	RequiredVars    []*CodegenProperty `json:"requiredVars"` // Required properties
	OptionalVars    []*CodegenProperty `json:"optionalVars"`
	ReadOnlyVars    []*CodegenProperty `json:"readOnlyVars"`
	ReadWriteVars   []*CodegenProperty `json:"readWriteVars"`
	ParentVars      []*CodegenProperty `json:"parentVars"`
	NonNullableVars []*CodegenProperty `json:"nonNullableVars"`

	// Required tracking
	Mandatory       []string `json:"mandatory"`    // Required property names (without parent's)
	AllMandatory    []string `json:"allMandatory"` // All required (with parent's)
	HasRequired     bool     `json:"hasRequired"`
	HasOptional     bool     `json:"hasOptional"`
	HasVars         bool     `json:"hasVars"`
	HasReadOnly     bool     `json:"hasReadOnly"`
	EmptyVars       bool     `json:"emptyVars"`
	HasEnums        bool     `json:"hasEnums"`
	HasOnlyReadOnly bool     `json:"hasOnlyReadOnly"`
	HasChildren     bool     `json:"hasChildren"`
	HasMoreModels   bool     `json:"hasMoreModels"`
	HasOneOf        bool     `json:"hasOneOf"`

	// Enum support
	AllowableValues map[string]any `json:"allowableValues"` // {"values": [...]}

	// Discriminator
	Discriminator                       *CodegenDiscriminator `json:"discriminator"`
	HasDiscriminatorWithNonEmptyMapping bool                  `json:"hasDiscriminatorWithNonEmptyMapping"`

	// Additional properties
	AdditionalPropertiesType   string           `json:"additionalPropertiesType"`
	IsAdditionalPropertiesTrue bool             `json:"isAdditionalPropertiesTrue"`
	AdditionalProperties       *CodegenProperty `json:"additionalProperties"`

	// Imports
	Imports []string `json:"imports"`

	// Vendor extensions
	VendorExtensions map[string]any `json:"vendorExtensions"`

	// XML
	XmlPrefix    string `json:"xmlPrefix"`
	XmlNamespace string `json:"xmlNamespace"`
	XmlName      string `json:"xmlName"`

	// Validation (from IJsonSchemaValidationProperties)
	Pattern          string   `json:"pattern"`
	Minimum          string   `json:"minimum"`
	Maximum          string   `json:"maximum"`
	MinLength        *int     `json:"minLength"`
	MaxLength        *int     `json:"maxLength"`
	MinItems         *int     `json:"minItems"`
	MaxItems         *int     `json:"maxItems"`
	MinProperties    *int     `json:"minProperties"`
	MaxProperties    *int     `json:"maxProperties"`
	UniqueItems      bool     `json:"uniqueItems"`
	ExclusiveMinimum bool     `json:"exclusiveMinimum"`
	ExclusiveMaximum bool     `json:"exclusiveMaximum"`
	MultipleOf       *float64 `json:"multipleOf"`

	// External documentation
	ExternalDocumentation map[string]any `json:"externalDocumentation"`

	// Items for array models
	Items *CodegenProperty `json:"items"`

	// Default value
	DefaultValue string `json:"defaultValue"`

	// JSON representation
	ModelJson string `json:"modelJson"`
}

// CodegenComposedSchemas holds composed schema references
type CodegenComposedSchemas struct {
	AllOf []*CodegenProperty `json:"allOf"`
	OneOf []*CodegenProperty `json:"oneOf"`
	AnyOf []*CodegenProperty `json:"anyOf"`
	Not   *CodegenProperty   `json:"not"`
}
