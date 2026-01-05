package codegen

// CodegenResponse represents an API response.
// It is the Go equivalent of org.openapitools.codegen.CodegenResponse.
type CodegenResponse struct {
	Code    string `json:"code"` // "200", "404", "default", etc.
	Message string `json:"message"`

	// Status code categories
	Is1xx     bool `json:"is1xx"`
	Is2xx     bool `json:"is2xx"`
	Is3xx     bool `json:"is3xx"`
	Is4xx     bool `json:"is4xx"`
	Is5xx     bool `json:"is5xx"`
	IsDefault bool `json:"isDefault"`

	// Type info
	DataType      string `json:"dataType"`
	BaseType      string `json:"baseType"`
	ContainerType string `json:"containerType"`

	// Type flags
	IsString         bool `json:"isString"`
	IsInteger        bool `json:"isInteger"`
	IsLong           bool `json:"isLong"`
	IsNumber         bool `json:"isNumber"`
	IsFloat          bool `json:"isFloat"`
	IsDouble         bool `json:"isDouble"`
	IsDecimal        bool `json:"isDecimal"`
	IsBoolean        bool `json:"isBoolean"`
	IsNumeric        bool `json:"isNumeric"`
	IsModel          bool `json:"isModel"`
	IsArray          bool `json:"isArray"`
	IsMap            bool `json:"isMap"`
	IsBinary         bool `json:"isBinary"`
	IsFile           bool `json:"isFile"`
	IsPrimitiveType  bool `json:"isPrimitiveType"`
	IsFreeFormObject bool `json:"isFreeFormObject"`
	IsAnyType        bool `json:"isAnyType"`
	IsUuid           bool `json:"isUuid"`
	IsEmail          bool `json:"isEmail"`
	IsNull           bool `json:"isNull"`
	IsVoid           bool `json:"isVoid"`

	// Headers
	Headers         []*CodegenProperty  `json:"headers"`
	ResponseHeaders []*CodegenParameter `json:"responseHeaders"`
	HasHeaders      bool                `json:"hasHeaders"`

	// Content
	Content map[string]*CodegenMediaType `json:"content"`

	// Nested
	Items                *CodegenProperty   `json:"items"`
	AdditionalProperties *CodegenProperty   `json:"additionalProperties"`
	Vars                 []*CodegenProperty `json:"vars"`
	ReturnProperty       *CodegenProperty   `json:"returnProperty"`

	// Examples
	Examples []map[string]any `json:"examples"`

	// Validation flags
	SimpleType    bool `json:"simpleType"`
	PrimitiveType bool `json:"primitiveType"`
	HasValidation bool `json:"hasValidation"`

	// Required vars
	RequiredVars []*CodegenProperty `json:"requiredVars"`

	// Vendor extensions
	VendorExtensions map[string]any `json:"vendorExtensions"`

	// Schema
	Schema     *CodegenProperty `json:"schema"`
	JsonSchema string           `json:"jsonSchema"`

	// Wildcard flag
	IsWildcard        bool   `json:"isWildcard"`
	WildcardCodeGroup string `json:"wildcardCodeGroup"`

	// Description
	Description          string `json:"description"`
	UnescapedDescription string `json:"unescapedDescription"`

	// Min/Max items
	MinItems    *int `json:"minItems"`
	MaxItems    *int `json:"maxItems"`
	UniqueItems bool `json:"uniqueItems"`
}
