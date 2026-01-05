package codegen

// CodegenParameter represents an operation parameter.
// It is the Go equivalent of org.openapitools.codegen.CodegenParameter.
type CodegenParameter struct {
	// Names
	BaseName  string `json:"baseName"`  // Original name
	ParamName string `json:"paramName"` // Language-specific name

	// Case variants
	NameInLowerCase  string `json:"nameInLowerCase"`
	NameInCamelCase  string `json:"nameInCamelCase"`
	NameInPascalCase string `json:"nameInPascalCase"`
	NameInSnakeCase  string `json:"nameInSnakeCase"`

	// Type
	DataType         string `json:"dataType"`
	DatatypeWithEnum string `json:"datatypeWithEnum"`
	DataFormat       string `json:"dataFormat"`
	BaseType         string `json:"baseType"`
	ContentType      string `json:"contentType"`

	// Location flags
	IsFormParam   bool `json:"isFormParam"`
	IsQueryParam  bool `json:"isQueryParam"`
	IsPathParam   bool `json:"isPathParam"`
	IsHeaderParam bool `json:"isHeaderParam"`
	IsCookieParam bool `json:"isCookieParam"`
	IsBodyParam   bool `json:"isBodyParam"`

	// Type flags
	IsContainer      bool `json:"isContainer"`
	IsArray          bool `json:"isArray"`
	IsMap            bool `json:"isMap"`
	IsPrimitiveType  bool `json:"isPrimitiveType"`
	IsModel          bool `json:"isModel"`
	IsString         bool `json:"isString"`
	IsInteger        bool `json:"isInteger"`
	IsLong           bool `json:"isLong"`
	IsNumber         bool `json:"isNumber"`
	IsFloat          bool `json:"isFloat"`
	IsDouble         bool `json:"isDouble"`
	IsDecimal        bool `json:"isDecimal"`
	IsBoolean        bool `json:"isBoolean"`
	IsNumeric        bool `json:"isNumeric"`
	IsByteArray      bool `json:"isByteArray"`
	IsBinary         bool `json:"isBinary"`
	IsFile           bool `json:"isFile"`
	IsDate           bool `json:"isDate"`
	IsDateType       bool `json:"isDateType"` // Computed: same as IsDate (for template compatibility)
	IsDateTime       bool `json:"isDateTime"`
	IsDateTimeType   bool `json:"isDateTimeType"` // Computed: same as IsDateTime (for template compatibility)
	IsUuid           bool `json:"isUuid"`
	IsUri            bool `json:"isUri"`
	IsEmail          bool `json:"isEmail"`
	IsFreeFormObject bool `json:"isFreeFormObject"`
	IsAnyType        bool `json:"isAnyType"`

	// Enum
	IsEnum          bool           `json:"isEnum"`
	IsEnumRef       bool           `json:"isEnumRef"`
	Enum            []string       `json:"enum"`
	EnumName        string         `json:"enumName"`
	AllowableValues map[string]any `json:"allowableValues"`

	// Validation
	Required         bool     `json:"required"`
	IsNullable       bool     `json:"isNullable"`
	IsDeprecated     bool     `json:"isDeprecated"`
	HasValidation    bool     `json:"hasValidation"`
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

	// Style
	Style                   string `json:"style"`
	IsExplode               bool   `json:"isExplode"`
	IsDeepObject            bool   `json:"isDeepObject"`
	IsMatrix                bool   `json:"isMatrix"`
	IsFormStyle             bool   `json:"isFormStyle"`
	IsSpaceDelimited        bool   `json:"isSpaceDelimited"`
	IsPipeDelimited         bool   `json:"isPipeDelimited"`
	IsAllowEmptyValue       bool   `json:"isAllowEmptyValue"`
	CollectionFormat        string `json:"collectionFormat"`
	IsCollectionFormatMulti bool   `json:"isCollectionFormatMulti"`

	// Nested
	Items                *CodegenProperty   `json:"items"`
	AdditionalProperties *CodegenProperty   `json:"additionalProperties"`
	Vars                 []*CodegenProperty `json:"vars"`
	RequiredVars         []*CodegenProperty `json:"requiredVars"`
	MostInnerItems       *CodegenProperty   `json:"mostInnerItems"`

	// Documentation
	Description          string         `json:"description"`
	UnescapedDescription string         `json:"unescapedDescription"`
	Example              string         `json:"example"`
	Examples             map[string]any `json:"examples"`
	JsonSchema           string         `json:"jsonSchema"`
	DefaultValue         string         `json:"defaultValue"`
	EnumDefaultValue     string         `json:"enumDefaultValue"`

	// Content (for complex parameters)
	Content map[string]*CodegenMediaType `json:"content"`

	// Vendor extensions
	VendorExtensions map[string]any `json:"vendorExtensions"`

	// HasVars and HasRequired
	HasVars     bool `json:"hasVars"`
	HasRequired bool `json:"hasRequired"`

	// MinProperties and MaxProperties
	MinProperties *int `json:"minProperties"`
	MaxProperties *int `json:"maxProperties"`
}

// CodegenMediaType represents media type content
type CodegenMediaType struct {
	Schema   *CodegenProperty `json:"schema"`
	Encoding map[string]any   `json:"encoding"`
}
