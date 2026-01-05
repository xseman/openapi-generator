package codegen

// CodegenDiscriminator represents a discriminator for polymorphic types.
// It is the Go equivalent of org.openapitools.codegen.CodegenDiscriminator.
type CodegenDiscriminator struct {
	PropertyName     string `json:"propertyName"`
	PropertyBaseName string `json:"propertyBaseName"`
	PropertyGetter   string `json:"propertyGetter"`
	PropertyType     string `json:"propertyType"`
	IsEnum           bool   `json:"isEnum"`

	// Mapping: discriminator value -> schema name
	Mapping      map[string]string `json:"mapping"`
	MappedModels []*MappedModel    `json:"mappedModels"`

	VendorExtensions map[string]any `json:"vendorExtensions"`
}

// MappedModel represents a discriminator mapping entry
type MappedModel struct {
	MappingName     string        `json:"mappingName"`     // Value in payload
	ModelName       string        `json:"modelName"`       // Schema/model name
	Model           *CodegenModel `json:"-"`               // Reference to model
	ExplicitMapping bool          `json:"explicitMapping"` // From spec vs inferred
}
