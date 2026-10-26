package typescript

import (
	"maps"
	"reflect"
	"regexp"
	"slices"
	"strconv"
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

	// byName are the generated models by class name: the ones with a
	// FromJSON and a ToJSON a converted value may call.
	byName map[string]*codegen.CodegenModel
}

// NewFetchGenerator creates a new TypeScript Fetch generator.
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

// GetName returns the generator name.
func (g *FetchGenerator) GetName() string {
	return "typescript-fetch"
}

// GetTag returns the generator type.
func (g *FetchGenerator) GetTag() generator.GeneratorType {
	return generator.GeneratorTypeClient
}

// GetHelp returns the help text.
func (g *FetchGenerator) GetHelp() string {
	return "Generates a TypeScript client library using Fetch API."
}

// ProcessOpts processes CLI options and initializes the generator.
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

// ToApiFilename returns the API file name with file naming convention applied.
func (g *FetchGenerator) ToApiFilename(name string) string {
	return g.convertUsingFileNamingConvention(g.BaseGenerator.ToApiFilename(name))
}

// ToModelFilename returns the model file name with file naming convention applied.
func (g *FetchGenerator) ToModelFilename(name string) string {
	return g.convertUsingFileNamingConvention(g.BaseGenerator.ToModelFilename(name))
}

// GetTypeDeclaration returns the type declaration for file/binary types.
func (g *FetchGenerator) GetTypeDeclaration(schemaType, format string) string {
	if schemaType == "file" || format == "binary" {
		return "Blob"
	}

	return g.BaseGenerator.GetTypeDeclaration(schemaType, format)
}

// EscapeReservedWord escapes reserved words.
func (g *FetchGenerator) EscapeReservedWord(name string) string {
	return g.BaseGenerator.EscapeReservedWord(name)
}

// PostProcessModels post-processes models for TypeScript-Fetch.
func (g *FetchGenerator) PostProcessModels(models []*codegen.CodegenModel) []*codegen.CodegenModel {
	g.byName = make(map[string]*codegen.CodegenModel, len(models))
	for _, cm := range models {
		g.byName[cm.Classname] = cm

		// An anyOf of models converts as a oneOf does, by the member it fits
		// best; without this it is an empty interface. One with an inline
		// member stays so: its type would leave that member out.
		if len(cm.AnyOf) > 0 && !cm.HasOneOf && !cm.HasVars && cm.VendorExtensions["x-any-of-inline"] == nil {
			for _, name := range cm.AnyOf {
				cm.OneOf = append(cm.OneOf, g.ToModelName(name))
			}

			cm.OneOfModels, cm.HasOneOf = slices.Sorted(slices.Values(cm.OneOf)), true
		}
	}

	for _, cm := range models {
		g.processCodeGenModel(cm)
		g.alias(cm)
		g.convertVars(cm, models)
		g.mapDescendants(cm, models)
		g.fitMembers(cm)
	}

	return models
}

// convertVars gives each property of cm its FromJSON and ToJSON value
// (x-ts-from-json, x-ts-to-json) and imports the models they call.
func (g *FetchGenerator) convertVars(cm *codegen.CodegenModel, models []*codegen.CodegenModel) {
	from := &converter{g: g, used: map[string]bool{}}
	to := &converter{g: g, to: true, used: from.used}

	cm.VendorExtensions = own(cm.VendorExtensions)

	for _, v := range cm.Vars {
		textBinary(v)

		v.VendorExtensions = own(v.VendorExtensions)

		v.VendorExtensions["x-ts-from-json"] = key(v.Name) + ": " + from.varFromJSON(v)
		v.VendorExtensions["x-ts-to-json"] = key(v.BaseName) + ": " + to.varToJSON(v)
	}

	// The keys next to the properties (additionalProperties) go through as
	// they are, or converted as their schema says: the properties' own keys
	// are left out, else a renamed one would be there twice, and so are those
	// of a model that extends cm, whose ToJSON and FromJSON call cm's.
	if cm.HasVars && (cm.AdditionalPropertiesType != "" || cm.IsAdditionalPropertiesTrue) {
		if cm.AdditionalPropertiesType == "" {
			cm.AdditionalPropertiesType = "any"
		}

		var wire, names []string

		for _, m := range append(descendants(cm, models), cm) {
			for _, v := range m.Vars {
				if k := strconv.Quote(v.BaseName); !slices.Contains(wire, k) {
					wire, names = append(wire, k), append(names, strconv.Quote(v.Name))
				}
			}
		}

		rest := func(c *converter, x string, keys []string) string {
			return c.expr(&codegen.CodegenProperty{IsMap: true, Items: cm.AdditionalProperties}, "otherKeys("+x+", ["+strings.Join(keys, ", ")+"])")
		}

		cm.VendorExtensions["x-ts-rest-from"] = rest(from, "json", wire)
		cm.VendorExtensions["x-ts-rest-to"] = rest(to, "value", names)
	}

	delete(from.used, cm.Classname)
	cm.Imports = addImports(cm.Imports, from.used)
}

// alias makes cm, a schema that only names an array, a map or a scalar
// (`Pets: {type: array, items: Pet}`, `Day: {format: date}`), a type alias
// that converts as its value does: x-ts-alias is the type, x-ts-alias-from
// and x-ts-alias-to the conversions. A property of that schema is typed as
// the value already; a body or a result is typed as the model.
func (g *FetchGenerator) alias(cm *codegen.CodegenModel) {
	if cm.HasVars || cm.HasOneOf || cm.IsEnum || len(cm.AllOf) > 0 || len(cm.AnyOf) > 0 || cm.Discriminator != nil {
		return
	}

	var value *codegen.CodegenProperty

	switch {
	case cm.IsArray && cm.Items != nil:
		value = &codegen.CodegenProperty{IsArray: true, IsContainer: true, Items: cm.Items, DataType: "Array<" + cm.Items.DataType + ">"}
	case cm.AdditionalProperties != nil:
		value = &codegen.CodegenProperty{IsMap: true, IsContainer: true, Items: cm.AdditionalProperties, DataType: "{ [key: string]: " + cm.AdditionalProperties.DataType + "; }"}
	case cm.IsPrimitiveType && cm.DataType != "" && cm.DataType != "any":
		value = &codegen.CodegenProperty{DataType: cm.DataType, IsDateType: cm.IsDate && cm.DataType == "Date", IsDateTimeType: cm.IsDateTime && cm.DataType == "Date"}
	default:
		return
	}

	from := &converter{g: g, used: map[string]bool{}}
	to := &converter{g: g, to: true, used: from.used}

	cm.VendorExtensions = own(cm.VendorExtensions)
	cm.VendorExtensions["x-ts-alias"] = value.DataType
	cm.VendorExtensions["x-ts-alias-from"] = from.expr(value, "json")
	cm.VendorExtensions["x-ts-alias-to"] = to.expr(value, "value")

	// The value's type names its models, converted or not.
	for p := value; p != nil; p = p.Items {
		if g.byName[p.DataType] != nil {
			from.used[p.DataType] = true
		}
	}

	delete(from.used, cm.Classname)
	cm.Imports = addImports(cm.Imports, from.used)
}

// descendants are the models that extend cm through allOf, at any depth.
func descendants(cm *codegen.CodegenModel, models []*codegen.CodegenModel) []*codegen.CodegenModel {
	var out []*codegen.CodegenModel

	for _, m := range models {
		if m != cm && slices.Contains(m.AllOf, cm.SchemaName) {
			out = append(append(out, m), descendants(m, models)...)
		}
	}

	return out
}

// mapDescendants completes the discriminator of cm, a parent the others
// extend with allOf: a value without a mapping names its schema, as OpenAPI
// defines, so each child is mapped by its schema name. The mapped models
// are named as their classes, and x-ts-mapped imports each once: two values
// may map to one model.
func (g *FetchGenerator) mapDescendants(cm *codegen.CodegenModel, models []*codegen.CodegenModel) {
	d := cm.Discriminator
	if d == nil {
		return
	}

	for _, m := range models {
		explicit := slices.ContainsFunc(d.MappedModels, func(mm *codegen.MappedModel) bool { return g.ToModelName(mm.ModelName) == m.Classname })
		if m != cm && !explicit && slices.Contains(m.AllOf, cm.SchemaName) {
			d.MappedModels = append(d.MappedModels, &codegen.MappedModel{MappingName: m.SchemaName, ModelName: m.SchemaName})
		}
	}

	var imports []string

	for _, mm := range d.MappedModels {
		mm.ModelName = g.ToModelName(mm.ModelName)
		if !slices.Contains(imports, mm.ModelName) {
			imports = append(imports, mm.ModelName)
		}
	}

	cm.VendorExtensions = own(cm.VendorExtensions)

	mapped := make([]map[string]string, 0, len(imports))
	for _, name := range imports {
		mapped = append(mapped, map[string]string{"modelName": name, "modelFilename": g.ToModelFilename(name)})
	}

	cm.VendorExtensions["x-ts-mapped"] = mapped
}

// textBinary types a binary property of a JSON model, or one in its
// containers, as a string: JSON holds no bytes, a server writes its text
// (pydantic's bytes), and a Blob would go out as {}.
func textBinary(p *codegen.CodegenProperty) {
	for c := p; c != nil; c = c.Items {
		if (c.IsBinary || c.IsFile) && !c.IsContainer {
			c.IsBinary, c.IsFile, c.IsString, c.IsPrimitiveType = false, false, true, true
		}

		for _, t := range []*string{&c.DataType, &c.Datatype, &c.DatatypeWithEnum, &c.BaseType} {
			*t = blobType.ReplaceAllString(*t, "string")
		}
	}
}

var blobType = regexp.MustCompile(`\bBlob\b`)

// own is a copy of the extensions ext to write to: the parser hands out the
// document's own map, which every property of one $ref shares.
func own(ext map[string]any) map[string]any {
	if ext = maps.Clone(ext); ext == nil {
		ext = map[string]any{}
	}

	return ext
}

// fitMembers gives a oneOf model without a discriminator the lists the
// runtime's bestFit picks its model member by (x-ts-fit-from, x-ts-fit-to):
// the first one whose required keys a value has would take a value of a
// later one with more keys, and drop them.
func (g *FetchGenerator) fitMembers(cm *codegen.CodegenModel) {
	if cm.Discriminator != nil || len(cm.OneOfModels) == 0 {
		return
	}

	// A member is a model, or the intersection of an allOf ("A & B"), whose
	// models all import.
	var members []fit

	for _, member := range cm.OneOf {
		var parts []*codegen.CodegenModel

		for name := range strings.SplitSeq(member, " & ") {
			if m := g.byName[name]; m != nil {
				parts = append(parts, m)
			}
		}

		if len(parts) == 0 || strings.Count(member, " & ")+1 != len(parts) {
			continue
		}

		members = append(members, fit{parts: parts})

		for _, m := range parts {
			if !slices.Contains(cm.OneOfModels, m.Classname) {
				cm.OneOfModels = append(cm.OneOfModels, m.Classname)
			}
		}
	}

	slices.Sort(cm.OneOfModels)

	from := &converter{g: g, used: map[string]bool{}}
	to := &converter{g: g, to: true, used: from.used}

	cm.VendorExtensions = own(cm.VendorExtensions)
	cm.VendorExtensions["x-ts-fit-from"] = from.fits(members)
	cm.VendorExtensions["x-ts-fit-to"] = to.fits(members)
}

// optional is 1 for an optional parameter, 0 for a required one.
func optional(p *codegen.CodegenParameter) int {
	if p.Required {
		return 0
	}

	return 1
}

// key is name as the key of an object literal. "__proto__" would set the
// prototype, not a property: it is computed.
func key(name string) string {
	if name == "__proto__" {
		return `["__proto__"]`
	}

	return strconv.Quote(name)
}

// addImports adds the names in used to imports, once, sorted.
func addImports(imports []string, used map[string]bool) []string {
	for name := range used {
		if !slices.Contains(imports, name) {
			imports = append(imports, name)
		}
	}

	slices.Sort(imports)

	return imports
}

// PostProcessOperations post-processes operations for TypeScript-Fetch.
func (g *FetchGenerator) PostProcessOperations(operations []*codegen.CodegenOperation) []*codegen.CodegenOperation {
	for _, op := range operations {
		// Positional arguments: no required one after an optional one.
		if !g.UseSingleRequestParameter {
			slices.SortStableFunc(op.AllParams, func(a, b *codegen.CodegenParameter) int {
				return optional(a) - optional(b)
			})
		}

		g.escapeOperationId(op)
		g.updateOperationParameterForEnum(op)
		g.addOperationObjectResponseInformation(op)
		g.convertOperation(op)
	}

	return operations
}

// convertOperation gives op the transformer of its JSON result
// (x-ts-result, absent when the JSON is the result) and its body the
// value sent (x-ts-body), and imports the models they call.
func (g *FetchGenerator) convertOperation(op *codegen.CodegenOperation) {
	from := &converter{g: g, rt: "runtime.", used: map[string]bool{}}
	to := &converter{g: g, to: true, rt: "runtime.", used: from.used}

	op.VendorExtensions = own(op.VendorExtensions)

	result := op.ReturnProperty
	if result == nil && op.ReturnType != "" {
		result = &codegen.CodegenProperty{Datatype: op.ReturnBaseType, IsModel: !op.ReturnTypeIsPrimitive}
	}

	if v := from.expr(result, "jsonValue"); v != "jsonValue" {
		op.VendorExtensions["x-ts-result"] = v
	}

	g.stream(op, from)

	if b := op.BodyParam; b != nil {
		x := `requestParameters["` + b.ParamName + `"]`

		v := to.expr(asProperty(b), x)
		if v != x && (b.IsContainer || b.IsDateType || b.IsDateTimeType) {
			v = x + " == null ? " + x + " : " + v
		}

		b.VendorExtensions = own(b.VendorExtensions)

		b.VendorExtensions["x-ts-body"] = v
	}

	for _, list := range [][]*codegen.CodegenParameter{op.QueryParams, op.HeaderParams, op.PathParams, op.CookieParams, op.FormParams} {
		for _, param := range list {
			g.paramValue(to, param)
		}
	}

	op.Imports = addImports(op.Imports, from.used)

	g.responseKinds(op)

	multipart := slices.ContainsFunc(op.Consumes, func(c map[string]string) bool { return strings.HasPrefix(c["mediaType"], "multipart/") })
	for _, param := range op.FormParams {
		multipart = multipart || param.IsFile
	}

	op.VendorExtensions["x-ts-multipart"] = multipart
	op.VendorExtensions["x-ts-cookies"] = len(op.CookieParams) > 0 ||
		slices.ContainsFunc(op.AuthMethods, func(a *codegen.CodegenSecurity) bool { return a.IsKeyInCookie })

	for _, param := range op.FormParams {
		formField(param, multipart)
	}
}

// paramValue gives param the value its serialization starts from
// (x-ts-value: dates as their text, a model as its JSON) and its style with
// the default for its location filled in (x-ts-style).
func (g *FetchGenerator) paramValue(to *converter, param *codegen.CodegenParameter) {
	param.VendorExtensions = own(param.VendorExtensions)

	param.VendorExtensions["x-ts-value"] = to.expr(asProperty(param), `requestParameters["`+param.ParamName+`"]!`)

	style := param.Style
	if style == "" {
		style = "form"
		if param.IsPathParam || param.IsHeaderParam {
			style = "simple"
		}
	}

	param.VendorExtensions["x-ts-style"] = style

	// A query object not in deepObject style: exploded form spreads its
	// properties as parameters, else it is one of key,value pairs.
	if param.IsQueryParam && !param.IsDeepObject && isObject(param) {
		param.VendorExtensions["x-ts-spread"] = param.IsExplode
		param.VendorExtensions["x-ts-styled"] = !param.IsExplode
	}
}

// isObject says whether param's value is an object. A $ref to an enum or a
// primitive is a model too, and its value no object.
func isObject(param *codegen.CodegenParameter) bool {
	return param.IsModel && !param.IsEnumRef && !param.IsPrimitiveType || param.IsMap || param.IsFreeFormObject
}

// formField says how form field param goes into the body: a file as it is
// (x-ts-file, a list of them x-ts-files), an object in a multipart body as
// a JSON part (x-ts-json), anything else as the fields its encoding writes
// (x-ts-fields, the arguments of the runtime's formFields after the name).
func formField(param *codegen.CodegenParameter, multipart bool) {
	value, _ := param.VendorExtensions["x-ts-value"].(string)
	style, _ := param.VendorExtensions["x-ts-style"].(string)
	object := isObject(param) || param.IsArray && param.Items != nil && (param.Items.IsModel && !param.Items.IsPrimitiveType || param.Items.IsFreeFormObject)

	switch {
	case param.IsFile && param.IsArray:
		param.VendorExtensions["x-ts-files"] = true
	case param.IsFile:
		param.VendorExtensions["x-ts-file"] = true
	case multipart && object:
		param.VendorExtensions["x-ts-json"] = value
	default:
		param.VendorExtensions["x-ts-fields"] = value + ", " + strconv.Quote(style) + ", " + strconv.FormatBool(param.IsExplode)
	}
}

// stream makes the result of op, whose 2xx response streams its items
// (x-stream), an AsyncIterable of them: of JSON Lines (application/jsonl,
// application/x-ndjson) a value per line, of a text/event-stream a
// ServerSentEvent per event, its data the item, parsed from JSON unless the
// item is text. x-ts-stream is the kind, x-ts-stream-item the conversion of
// one item v. Another streamed media type stays without a result.
func (g *FetchGenerator) stream(op *codegen.CodegenOperation, from *converter) {
	for _, r := range op.Responses {
		item, _ := r.VendorExtensions["x-stream-item"].(*codegen.CodegenProperty)
		mediaType, _ := r.VendorExtensions["x-stream"].(string)
		mediaType, _, _ = strings.Cut(mediaType, ";")

		kind := map[string]string{"application/jsonl": "lines", "application/x-ndjson": "lines", "text/event-stream": "events"}[mediaType]
		if !r.Is2xx || item == nil || kind == "" || op.ReturnType != "" {
			continue
		}

		textBinary(item)

		x, yields := "v", item.DataType
		if kind == "events" {
			yields = "runtime.ServerSentEvent<" + yields + ">"

			if !item.IsString {
				x = "JSON.parse(v)"
			}
		}

		op.ReturnType = "AsyncIterable<" + yields + ">"
		op.VendorExtensions["x-ts-stream"] = kind
		op.VendorExtensions["x-ts-stream-item"] = from.expr(item, x)

		// The item's type names its models, converted or not.
		for p := item; p != nil; p = p.Items {
			for _, m := range p.ComposedModels {
				from.used[g.ToModelName(m)] = true
			}

			if g.byName[p.DataType] != nil {
				from.used[p.DataType] = true
			}
		}

		return
	}
}

// responseKinds marks how op reads its result: x-ts-text when the result
// may come as text (a string, or any body the server does not send as
// JSON), x-ts-any when it may also come as a file (an any the server
// sends as an image or as octets), x-ts-other-2xx the statuses whose body is of another type than the
// result, isResponseOptional when a 2xx comes without a body (a 204 beside
// a 200, the answer to a HEAD), which reads as null.
func (g *FetchGenerator) responseKinds(op *codegen.CodegenOperation) {
	if op.ReturnType == "" || op.VendorExtensions["x-ts-stream"] != nil {
		return
	}

	op.VendorExtensions["x-ts-text"] = op.ReturnType == "string" || op.ReturnType == "any"
	op.VendorExtensions["x-ts-any"] = op.ReturnType == "any"
	op.IsResponseOptional = op.HttpMethod == "HEAD"

	var other []string

	for _, r := range op.Responses {
		if _, err := strconv.Atoi(r.Code); err != nil || !r.Is2xx {
			continue
		}

		switch {
		case r.DataType == "":
			op.IsResponseOptional = true
		case op.ReturnType != "any" && r.DataType != op.ReturnType && (r.ReturnProperty == nil || !reflect.DeepEqual(r.ReturnProperty, op.ReturnProperty)):
			other = append(other, r.Code)
		}
	}

	if len(other) > 0 {
		op.VendorExtensions["x-ts-other-2xx"] = "[" + strings.Join(other, ", ") + "]"
	}
}

// processCodeGenModel processes a model for TypeScript-Fetch specific transformations.
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

// processCodegenProperty processes a property for TypeScript-Fetch specific transformations.
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

// escapeOperationId escapes operation IDs that conflict with imports or naming patterns.
func (g *FetchGenerator) escapeOperationId(op *codegen.CodegenOperation) {
	// Check for conflict with "Request" suffix import
	if slices.Contains(op.Imports, op.OperationIdCamelCase+"Request") {
		op.OperationIdCamelCase += "Operation"
		op.OperationIdLowerCase += "operation"
		op.OperationIdSnakeCase += "_operation"
		op.Nickname += "Operation"
	}

	// A method of BaseAPI would be overridden: the generated methods call
	// this.request, a method named request would call itself.
	if baseAPIMembers[op.Nickname] {
		op.OperationIdCamelCase += "Operation"
		op.OperationIdLowerCase += "operation"
		op.OperationIdSnakeCase += "_operation"
		op.Nickname += "Operation"
	}

	// Check if operationId ends with "Raw" which would conflict with the internal method naming pattern
	// The template generates methodNameRaw() for internal methods, so an operationId like "fooRaw"
	// would conflict with the internal method of operation "foo"
	if strings.HasSuffix(op.OperationIdCamelCase, "Raw") {
		op.OperationIdCamelCase += "Method"
		op.OperationIdLowerCase += "method"
		op.OperationIdSnakeCase += "_method"
		op.Nickname += "Method"
	}
}

// updateOperationParameterForEnum updates parameter enum names.
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

// addOperationObjectResponseInformation handles object response types.
func (g *FetchGenerator) addOperationObjectResponseInformation(op *codegen.CodegenOperation) {
	if op.ReturnType == "object" {
		op.IsMap = true
		op.ReturnSimpleType = false
	}
}

// convertUsingFileNamingConvention applies file naming convention.
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

// baseAPIMembers are the methods and fields of the runtime's BaseAPI, which
// an operation's method must not override.
var baseAPIMembers = map[string]bool{
	"request": true, "fetchApi": true, "configuration": true, "middleware": true, "clone": true,
	"withMiddleware": true, "withPreMiddleware": true, "withPostMiddleware": true, "isJsonMime": true,
	"createFetchParams": true,
}

// addExtraReservedWords adds typescript-fetch specific reserved words: the
// runtime's exports and the DOM types an API file uses, which a model of
// the same name would shadow. IsReservedWord looks them up lower-cased.
func (g *FetchGenerator) addExtraReservedWords() {
	extraWords := []string{
		"BASE_PATH", "BaseAPI", "RequiredError", "COLLECTION_FORMATS",
		"FetchAPI", "ConfigurationParameters", "Configuration", "configuration",
		"HTTPMethod", "HTTPHeaders", "HTTPQuery", "HTTPBody",
		"ModelPropertyNaming", "FetchParams", "RequestOpts", "exists",
		"RequestContext", "ResponseContext", "Middleware", "ApiResponse",
		"ResponseTransformer", "JSONApiResponse", "VoidApiResponse",
		"BlobApiResponse", "TextApiResponse", "Index",
		"RequestInit", "FormData", "URLSearchParams",
	}
	for _, word := range extraWords {
		g.ReservedWords[strings.ToLower(word)] = true
	}
}
