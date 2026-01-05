package codegen

// CodegenProperty represents a property within a model.
// It is the Go equivalent of org.openapitools.codegen.CodegenProperty.
type CodegenProperty struct {
	// Names
	Name     string `json:"name"`     // Language-specific variable name
	BaseName string `json:"baseName"` // Original property name from OAS
	Getter   string `json:"getter"`
	Setter   string `json:"setter"`

	// Case variants
	NameInLowerCase  string `json:"nameInLowerCase"`
	NameInCamelCase  string `json:"nameInCamelCase"`
	NameInPascalCase string `json:"nameInPascalCase"`
	NameInSnakeCase  string `json:"nameInSnakeCase"`

	// Type info - note: templates use lowercase "datatype" so we provide both
	OpenApiType         string `json:"openApiType"` // Type from OAS (string, integer, etc.)
	DataType            string `json:"dataType"`    // Language-specific type
	Datatype            string `json:"datatype"`    // Alias for templates using lowercase
	BaseType            string `json:"baseType"`    // Base type (for containers)
	ComplexType         string `json:"complexType"` // Complex/model type name
	DataFormat          string `json:"dataFormat"`  // Format (date-time, uuid, etc.)
	DatatypeWithEnum    string `json:"datatypeWithEnum"`
	ContainerType       string `json:"containerType"`       // "array", "map", "set"
	ContainerTypeMapped string `json:"containerTypeMapped"` // Language-specific container

	// Documentation
	Title                string `json:"title"`
	Description          string `json:"description"`
	UnescapedDescription string `json:"unescapedDescription"`
	Example              string `json:"example"`
	JsonSchema           string `json:"jsonSchema"`

	// Flags
	Required    bool `json:"required"`
	Deprecated  bool `json:"deprecated"`
	IsReadOnly  bool `json:"isReadOnly"`
	IsWriteOnly bool `json:"isWriteOnly"`
	IsNullable  bool `json:"isNullable"`

	// Type flags
	IsPrimitiveType  bool `json:"isPrimitiveType"`
	IsModel          bool `json:"isModel"`
	IsContainer      bool `json:"isContainer"`
	IsArray          bool `json:"isArray"`
	IsMap            bool `json:"isMap"`
	IsOptional       bool `json:"isOptional"`
	IsString         bool `json:"isString"`
	IsNumeric        bool `json:"isNumeric"`
	IsInteger        bool `json:"isInteger"`
	IsLong           bool `json:"isLong"`
	IsShort          bool `json:"isShort"`
	IsNumber         bool `json:"isNumber"`
	IsFloat          bool `json:"isFloat"`
	IsDouble         bool `json:"isDouble"`
	IsDecimal        bool `json:"isDecimal"`
	IsByteArray      bool `json:"isByteArray"`
	IsBinary         bool `json:"isBinary"`
	IsFile           bool `json:"isFile"`
	IsBoolean        bool `json:"isBoolean"`
	IsDate           bool `json:"isDate"`
	IsDateType       bool `json:"isDateType"` // Computed: same as IsDate (for template compatibility)
	IsDateTime       bool `json:"isDateTime"`
	IsDateTimeType   bool `json:"isDateTimeType"` // Computed: same as IsDateTime (for template compatibility)
	IsUuid           bool `json:"isUuid"`
	IsUri            bool `json:"isUri"`
	IsEmail          bool `json:"isEmail"`
	IsPassword       bool `json:"isPassword"`
	IsNull           bool `json:"isNull"`
	IsFreeFormObject bool `json:"isFreeFormObject"`
	IsAnyType        bool `json:"isAnyType"`
	IsEnum           bool `json:"isEnum"`
	IsInnerEnum      bool `json:"isInnerEnum"` // Inline enum
	IsEnumRef        bool `json:"isEnumRef"`   // Reference to enum model

	// Enum
	Enum            []string       `json:"enum"`
	EnumName        string         `json:"enumName"`
	AllowableValues map[string]any `json:"allowableValues"`

	// Nested types
	Items                *CodegenProperty   `json:"items"` // For arrays/maps
	AdditionalProperties *CodegenProperty   `json:"additionalProperties"`
	MostInnerItems       *CodegenProperty   `json:"mostInnerItems"`
	Vars                 []*CodegenProperty `json:"vars"` // For object types
	RequiredVars         []*CodegenProperty `json:"requiredVars"`

	// Default value
	DefaultValue          string `json:"defaultValue"`
	DefaultValueWithParam string `json:"defaultValueWithParam"`

	// Validation
	Pattern          string   `json:"pattern"`
	Minimum          string   `json:"minimum"`
	Maximum          string   `json:"maximum"`
	MinLength        *int     `json:"minLength"`
	MaxLength        *int     `json:"maxLength"`
	MinItems         *int     `json:"minItems"`
	MaxItems         *int     `json:"maxItems"`
	UniqueItems      bool     `json:"uniqueItems"`
	ExclusiveMinimum bool     `json:"exclusiveMinimum"`
	ExclusiveMaximum bool     `json:"exclusiveMaximum"`
	MultipleOf       *float64 `json:"multipleOf"`
	HasValidation    bool     `json:"hasValidation"`
	Min              string   `json:"min"`
	Max              string   `json:"max"`

	// Inheritance
	IsInherited         bool   `json:"isInherited"`
	IsNew               bool   `json:"isNew"` // Property overrides parent
	IsOverridden        *bool  `json:"isOverridden"`
	IsSelfReference     bool   `json:"isSelfReference"`
	IsCircularReference bool   `json:"isCircularReference"`
	IsDiscriminator     bool   `json:"isDiscriminator"`
	DiscriminatorValue  string `json:"discriminatorValue"`

	// Vendor extensions
	VendorExtensions map[string]any `json:"vendorExtensions"`

	// XML
	IsXmlAttribute bool   `json:"isXmlAttribute"`
	IsXmlWrapped   bool   `json:"isXmlWrapped"`
	XmlPrefix      string `json:"xmlPrefix"`
	XmlName        string `json:"xmlName"`
	XmlNamespace   string `json:"xmlNamespace"`

	// Composition
	ComposedSchemas *CodegenComposedSchemas `json:"composedSchemas"`
}
