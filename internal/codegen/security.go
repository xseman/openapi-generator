package codegen

// CodegenSecurity represents authentication/authorization configuration.
// It is the Go equivalent of org.openapitools.codegen.CodegenSecurity.
type CodegenSecurity struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`   // "apiKey", "http", "oauth2", "openIdConnect"
	Scheme      string `json:"scheme"` // "basic", "bearer", etc.

	// Type flags
	IsBasic         bool `json:"isBasic"`
	IsBasicBasic    bool `json:"isBasicBasic"`  // Basic auth
	IsBasicBearer   bool `json:"isBasicBearer"` // Bearer token
	IsApiKey        bool `json:"isApiKey"`
	IsOAuth         bool `json:"isOAuth"`
	IsOpenId        bool `json:"isOpenId"`
	IsHttpSignature bool `json:"isHttpSignature"`

	// Bearer
	BearerFormat string `json:"bearerFormat"`

	// API Key
	KeyParamName  string `json:"keyParamName"`
	IsKeyInQuery  bool   `json:"isKeyInQuery"`
	IsKeyInHeader bool   `json:"isKeyInHeader"`
	IsKeyInCookie bool   `json:"isKeyInCookie"`

	// OAuth
	Flow             string           `json:"flow"`
	AuthorizationUrl string           `json:"authorizationUrl"`
	TokenUrl         string           `json:"tokenUrl"`
	RefreshUrl       string           `json:"refreshUrl"`
	Scopes           []map[string]any `json:"scopes"`
	HasScopes        bool             `json:"hasScopes"`
	IsCode           bool             `json:"isCode"`
	IsPassword       bool             `json:"isPassword"`
	IsApplication    bool             `json:"isApplication"`
	IsImplicit       bool             `json:"isImplicit"`

	// OpenID
	OpenIdConnectUrl string `json:"openIdConnectUrl"`

	// Vendor extensions
	VendorExtensions map[string]any `json:"vendorExtensions"`
}
