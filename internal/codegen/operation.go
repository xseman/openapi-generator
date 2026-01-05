package codegen

// CodegenOperation represents an API operation.
// It is the Go equivalent of org.openapitools.codegen.CodegenOperation.
type CodegenOperation struct {
	// Identity
	OperationId          string `json:"operationId"`
	OperationIdOriginal  string `json:"operationIdOriginal"`
	OperationIdLowerCase string `json:"operationIdLowerCase"`
	OperationIdCamelCase string `json:"operationIdCamelCase"`
	OperationIdSnakeCase string `json:"operationIdSnakeCase"`

	// HTTP
	Path       string `json:"path"`
	HttpMethod string `json:"httpMethod"`
	BaseName   string `json:"baseName"` // Tag name
	Nickname   string `json:"nickname"` // Method name

	// Documentation
	Summary        string `json:"summary"`
	Notes          string `json:"notes"`
	UnescapedNotes string `json:"unescapedNotes"`

	// Parameters (categorized)
	AllParams      []*CodegenParameter `json:"allParams"`
	BodyParams     []*CodegenParameter `json:"bodyParams"`
	PathParams     []*CodegenParameter `json:"pathParams"`
	QueryParams    []*CodegenParameter `json:"queryParams"`
	HeaderParams   []*CodegenParameter `json:"headerParams"`
	FormParams     []*CodegenParameter `json:"formParams"`
	CookieParams   []*CodegenParameter `json:"cookieParams"`
	RequiredParams []*CodegenParameter `json:"requiredParams"`
	OptionalParams []*CodegenParameter `json:"optionalParams"`
	BodyParam      *CodegenParameter   `json:"bodyParam"`

	// Response
	ReturnType      string             `json:"returnType"`
	ReturnBaseType  string             `json:"returnBaseType"`
	ReturnContainer string             `json:"returnContainer"`
	ReturnFormat    string             `json:"returnFormat"`
	ReturnProperty  *CodegenProperty   `json:"returnProperty"`
	Responses       []*CodegenResponse `json:"responses"`
	ResponseHeaders []*CodegenProperty `json:"responseHeaders"`
	DefaultResponse string             `json:"defaultResponse"`

	// Content types
	Consumes    []map[string]string `json:"consumes"` // [{"mediaType": "application/json"}]
	Produces    []map[string]string `json:"produces"`
	HasConsumes bool                `json:"hasConsumes"`
	HasProduces bool                `json:"hasProduces"`

	// Prioritized content types
	PrioritizedContentTypes []map[string]string `json:"prioritizedContentTypes"`

	// Flags
	IsDeprecated           bool `json:"isDeprecated"`
	IsCallbackRequest      bool `json:"isCallbackRequest"`
	HasAuthMethods         bool `json:"hasAuthMethods"`
	HasOptionalParams      bool `json:"hasOptionalParams"`
	ReturnTypeIsPrimitive  bool `json:"returnTypeIsPrimitive"`
	ReturnSimpleType       bool `json:"returnSimpleType"`
	IsMap                  bool `json:"isMap"`
	IsArray                bool `json:"isArray"`
	IsMultipart            bool `json:"isMultipart"`
	IsVoid                 bool `json:"isVoid"`
	IsResponseBinary       bool `json:"isResponseBinary"`
	IsResponseFile         bool `json:"isResponseFile"`
	IsResponseOptional     bool `json:"isResponseOptional"`
	HasReference           bool `json:"hasReference"`
	HasErrorResponseObject bool `json:"hasErrorResponseObject"`
	UniqueItems            bool `json:"uniqueItems"`
	SubresourceOperation   bool `json:"subresourceOperation"`

	// Security
	AuthMethods []*CodegenSecurity `json:"authMethods"`

	// Servers
	Servers []*CodegenServer `json:"servers"`

	// Callbacks
	Callbacks []*CodegenCallback `json:"callbacks"`

	// Examples
	Examples            []map[string]string `json:"examples"`
	RequestBodyExamples []map[string]string `json:"requestBodyExamples"`

	// Discriminator
	Discriminator *CodegenDiscriminator `json:"discriminator"`

	// Imports
	Imports []string `json:"imports"`

	// Tags
	Tags []map[string]string `json:"tags"`

	// Vendor extensions
	VendorExtensions map[string]any `json:"vendorExtensions"`

	// External docs
	ExternalDocs map[string]any `json:"externalDocs"`
}

// CodegenServer represents server configuration
type CodegenServer struct {
	URL         string         `json:"url"`
	Description string         `json:"description"`
	Variables   map[string]any `json:"variables"`
}

// CodegenCallback represents a callback
type CodegenCallback struct {
	Name       string              `json:"name"`
	Operations []*CodegenOperation `json:"operations"`
}
