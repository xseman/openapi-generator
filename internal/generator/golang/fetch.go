package golang

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/xseman/openapi-generator/internal/codegen"
	"github.com/xseman/openapi-generator/internal/config"
	"github.com/xseman/openapi-generator/internal/generator"
)

// FetchGenerator implements the go-fetch generator: a client in the shape of
// typescript-fetch (an API type per tag, a Request, Raw and typed method per
// operation) that imports only the standard library.
//
// Templates read what they render from vendorExtensions keys x-go-*, which
// PostProcessModels and PostProcessOperations fill in, and the api template
// also from goDeclareParams, which the pipeline sets.
type FetchGenerator struct {
	*BaseGenerator

	GoConfig *config.GoFetchConfig

	// containers are the models declared as a slice, map or any alias: like
	// a slice, a field of one needs no pointer to be optional.
	containers map[string]bool

	// types are the type names declared so far, inline ones included;
	// opTypes the inline type names of operations, by operation and base.
	types   map[string]bool
	opTypes map[string]string

	// refs are the enum and union models by schema name, for a property
	// that refers to one: the parser types it as the enum or union inline.
	refs map[string]string

	// enums are the enum models by class: a parameter that refers to one
	// takes its type; one that refers to a union stays raw JSON, which the
	// runtime sends as a scalar.
	enums map[string]bool

	// nullable are the nullable models by class: a result that refers to
	// one is a pointer, nil for a null body.
	nullable map[string]bool

	// required are the quoted required properties of each struct model, which
	// a union's As method checks a value for; pinned its properties an enum
	// of one value fixes (see pinned).
	required map[string]string
	pinned   map[string][]map[string]any
}

// NewFetchGenerator creates a new go-fetch generator.
func NewFetchGenerator() *FetchGenerator {
	g := &FetchGenerator{BaseGenerator: NewBaseGenerator(), containers: map[string]bool{}, types: maps.Clone(RuntimeNames), opTypes: map[string]string{}, refs: map[string]string{}, enums: map[string]bool{}, nullable: map[string]bool{}, required: map[string]string{}, pinned: map[string][]map[string]any{}}
	g.ApiTemplateFiles["api.mustache"] = ".go"
	g.ModelTemplateFiles["model.mustache"] = ".go"

	return g
}

// GetName returns the generator name.
func (g *FetchGenerator) GetName() string { return "go-fetch" }

// GetTag returns the generator type.
func (g *FetchGenerator) GetTag() generator.GeneratorType { return generator.GeneratorTypeClient }

// GetHelp returns the help text.
func (g *FetchGenerator) GetHelp() string {
	return "Generates a Go client library that imports only the standard library."
}

// ProcessOpts processes options and initializes the generator.
func (g *FetchGenerator) ProcessOpts() error {
	cfg := g.GoConfig
	if cfg == nil {
		cfg = config.NewGoFetchConfig()
	}

	pkg := packageName(cfg.PackageName)

	module := cfg.ModuleName
	if module == "" {
		module = pkg
	}

	g.ApiPackage, g.ModelPackage = "", ""

	g.AdditionalProperties["packageName"] = pkg
	g.AdditionalProperties["moduleName"] = module
	g.AdditionalProperties["withGoMod"] = cfg.WithGoMod

	// ponytail: runtime.go's BasePath is servers[0].url as written, server
	// variables are not substituted; a caller with a templated URL sets
	// Configuration.BasePath. The upgrade is filling in their defaults.
	g.SupportingFiles = append(g.SupportingFiles,
		generator.NewSupportingFile("runtime.mustache", "", "runtime.go"),
		generator.NewSupportingFile("README.mustache", "", "README.md"),
	)
	if cfg.WithGoMod {
		g.SupportingFiles = append(g.SupportingFiles, generator.NewSupportingFile("gomod.mustache", "", "go.mod"))
	}

	return nil
}

// packageName keeps the lower-case letters and digits of name, "openapi"
// when none are left; a keyword gets an api suffix.
func packageName(name string) string {
	var b strings.Builder

	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9' && b.Len() > 0) {
			b.WriteRune(r)
		}
	}

	switch pkg := b.String(); {
	case pkg == "":
		return "openapi"
	case keywords[pkg]:
		return pkg + "api"
	default:
		return pkg
	}
}

// PostProcessModels types every model for Go: struct fields with allOf
// flattened, enums, aliases and union wrappers.
func (g *FetchGenerator) PostProcessModels(models []*codegen.CodegenModel) []*codegen.CodegenModel {
	bySchema := make(map[string]*codegen.CodegenModel, len(models))
	for _, m := range models {
		bySchema[m.SchemaName] = m
	}

	for _, m := range models {
		if len(m.AllOf) > 0 {
			flattenAllOf(m, bySchema)
		}
	}

	kinds := make(map[string]string, len(models))

	for _, m := range models {
		kind := modelKind(m)
		kinds[m.Classname] = kind
		g.types[m.Classname] = true

		if kind == "enum" || kind == "union" {
			g.refs[m.SchemaName] = m.Classname
		}

		g.enums[m.Classname] = kind == "enum"
		g.nullable[m.Classname] = m.IsNullable

		g.required[m.Classname] = requiredKeys(m.AllVars)
		g.pinned[m.Classname] = pinned(m.Classname, m.AllVars)

		if kind == "alias" {
			if t := aliasType(m); isContainer(t) {
				g.containers[m.Classname] = true
			}
		}
	}

	fields := make(map[string][]map[string]any)

	for _, m := range models {
		ext := clone(m.VendorExtensions)
		ext["x-go-doc"] = docLines(m.Description)
		ext["x-go-enums"] = []map[string]any{}
		ext["x-go-"+kinds[m.Classname]] = true

		switch kinds[m.Classname] {
		case "struct":
			fields[m.Classname] = g.structFields(m, ext, g.modelTypeName)
			fields[m.Classname] = extraField(m, fields[m.Classname], ext)
			ext["x-go-fields"] = fields[m.Classname]

			inline, _ := ext["x-go-inline"].([]map[string]any)
			for _, decl := range inline {
				if decl["union"] == true {
					continue
				}

				name, _ := decl["name"].(string)
				fields[name], _ = decl["fields"].([]map[string]any)
				kinds[name] = "struct"
			}

		case "enum":
			ext["x-go-enums"] = []map[string]any{enumDecl(m.Classname, EnumBaseType(m.AllowableValues, m.IsBoolean), m.AllowableValues, nil)}
		case "alias":
			ext["x-go-alias-type"] = aliasType(m)
		case "union":
			ext["x-go-doc"] = docLines(m.Description, m.Classname+" holds one of its variants as raw JSON:\nthe As methods decode it, the From methods set it.")
			g.unionExtensions(m, ext, bySchema)
		}

		m.VendorExtensions = ext
	}

	breakCycles(fields, kinds)

	return models
}

// modelKind is how a model is declared: union (oneOf/anyOf), enum, struct
// (properties or allOf) or alias of a slice, map or scalar type.
//
// ponytail: a discriminator without oneOf (allOf inheritance) makes no
// union: the parent stays a plain struct; the upgrade is a union wrapper for
// the parent too.
func modelKind(m *codegen.CodegenModel) string {
	switch {
	case len(m.OneOf) > 0 || len(m.AnyOf) > 0:
		return "union"
	case m.IsEnum && EnumBaseType(m.AllowableValues, m.IsBoolean) != "any":
		return "enum"
	case len(m.AllVars) > 0 || len(m.AllOf) > 0:
		return "struct"
	}

	return "alias"
}

// aliasType is the type a model without properties stands for.
func aliasType(m *codegen.CodegenModel) string {
	switch {
	case m.IsArray:
		return "[]" + itemType(m.Items, "")
	case m.AdditionalProperties != nil:
		return "map[string]" + itemType(m.AdditionalProperties, "")
	case m.IsString:
		return "string"
	case m.IsInteger:
		return "int"
	case m.IsNumber:
		return "float64"
	case m.IsBoolean:
		return "bool"
	case m.DataType == "any" || m.IsAdditionalPropertiesTrue:
		return "map[string]any"
	}

	return "any"
}

// extraField adds to a struct that takes additionalProperties next to its
// properties a map field for the keys it does not declare, and describes in
// ext the (Un)MarshalJSON that moves them in and out of it.
func extraField(m *codegen.CodegenModel, fields []map[string]any, ext map[string]any) []map[string]any {
	if !m.IsAdditionalPropertiesTrue && m.AdditionalProperties == nil {
		return fields
	}

	value := "any"
	if m.AdditionalProperties != nil {
		value = itemType(m.AdditionalProperties, "")
	}

	taken := make(map[string]int)

	declared := make([]string, 0, len(m.AllVars))
	for _, v := range m.AllVars {
		declared = append(declared, strconv.Quote(v.BaseName))
	}

	for _, f := range fields {
		name, _ := f["name"].(string)
		taken[name]++
	}

	name := uniqueName(taken, "AdditionalProperties")
	fields = append(fields, map[string]any{
		"name": name, "type": "map[string]" + value, "tag": "`json:\"-\"`",
		"doc": []string{"// " + name + " holds the keys the schema does not declare."},
	})
	ext["x-go-extra"] = map[string]any{"field": name, "value": value, "declared": strings.Join(declared, ", ")}

	return fields
}

// isContainer reports whether a type is nil-able as it is (slice, map, any,
// raw JSON), so an optional field of it needs no pointer.
func isContainer(t string) bool {
	return strings.HasPrefix(t, "[]") || strings.HasPrefix(t, "map[") ||
		t == "any" || t == "json.RawMessage" || t == "io.Reader"
}

// flattenAllOf gives a model built with allOf its ancestors' properties: the
// parser records the members only by name. Ancestors come first, in allOf
// order; a property declared again nearer the model replaces the inherited
// one in place, carrying its own required.
func flattenAllOf(m *codegen.CodegenModel, bySchema map[string]*codegen.CodegenModel) {
	vars := allVars(m, bySchema, map[string]bool{m.SchemaName: true})

	m.AllVars = vars
	m.RequiredVars = slices.DeleteFunc(slices.Clone(vars), func(v *codegen.CodegenProperty) bool { return !v.Required })
	m.OptionalVars = slices.DeleteFunc(slices.Clone(vars), func(v *codegen.CodegenProperty) bool { return v.Required })
	m.ReadOnlyVars = slices.DeleteFunc(slices.Clone(vars), func(v *codegen.CodegenProperty) bool { return !v.IsReadOnly })
}

func allVars(m *codegen.CodegenModel, bySchema map[string]*codegen.CodegenModel, visiting map[string]bool) []*codegen.CodegenProperty {
	var out []*codegen.CodegenProperty

	at := make(map[string]int)
	add := func(v *codegen.CodegenProperty) {
		if i, ok := at[v.BaseName]; ok {
			out[i] = v
			return
		}

		at[v.BaseName] = len(out)
		out = append(out, v)
	}

	for _, name := range m.AllOf {
		parent := bySchema[name]
		if parent == nil || visiting[name] {
			continue
		}

		visiting[name] = true

		for _, v := range allVars(parent, bySchema, visiting) {
			add(v)
		}

		delete(visiting, name)
	}

	for _, v := range m.Vars {
		add(v)
	}

	return out
}

// structFields types the properties of a struct model and declares its
// inline enums and objects in ext, named by typeName.
func (g *FetchGenerator) structFields(m *codegen.CodegenModel, ext map[string]any, typeName func(string) string) []map[string]any {
	var (
		fields []map[string]any
		enums  []map[string]any
		inline []map[string]any
	)

	names := make(map[string]int)
	enumNames := make(map[string]string)

	for _, v := range m.AllVars {
		named := ""

		what := "the " + v.BaseName + " property of " + m.Classname
		if v.IsArray || v.IsMap {
			what = "an item of " + what
		}

		if obj := inlineObject(v); obj != nil {
			named = typeName(m.Classname + v.Name)
			decls, nested := g.inlineStructs(named, what, obj.Description, obj.Vars, typeName)
			inline = append(inline, decls...)
			enums = append(enums, nested...)
		}

		if u := inlineUnion(v); u != nil {
			named = typeName(m.Classname + v.Name)
			decls, nested := g.unionDecl(named, what, u.Description, u.ComposedSchemas, typeName)
			inline = append(inline, decls...)
			enums = append(enums, nested...)
		}

		if ref := g.refModel(v.Ref); ref != "" {
			named = ref
		} else if v.IsEnum && EnumBaseType(v.AllowableValues, v.IsBoolean) != "any" {
			base := m.Classname + exported(v.EnumName, "")

			named = enumNames[base]
			if named == "" {
				named = typeName(base)
				enumNames[base] = named

				elem := v
				for elem.IsArray && elem.Items != nil {
					elem = elem.Items
				}

				enums = append(enums, enumDecl(named, EnumBaseType(v.AllowableValues, elem.IsBoolean), v.AllowableValues, v))
			}
		}

		t := GoType(v, named)
		ref := ""
		tag := v.BaseName

		optional := !v.Required || v.IsReadOnly
		container := isContainer(t) || g.containers[t]

		switch {
		case optional && v.IsNullable:
			if !container {
				ref = t
			}

			t = "Opt[" + t + "]"
			tag += ",omitzero"

		case optional && container:
			// nil is absent; an empty slice or map is still sent, as [] / {}.
			tag += ",omitzero"
		case optional:
			t = "*" + t
			tag += ",omitempty"

		case v.IsNullable && !container:
			t = "*" + t
		case !container:
			ref = t
		}

		// ponytail: a required slice or map left nil encodes as null, which a
		// server may refuse; the upgrade is a MarshalJSON that sends [] / {}.

		fields = append(fields, map[string]any{
			"name": uniqueName(names, v.Name),
			"type": t,
			"tag":  jsonTag(tag),
			"doc":  docLines(v.Description),
			"ref":  ref,
		})
	}

	ext["x-go-enums"] = enums
	ext["x-go-inline"] = inline

	return fields
}

// inlineObject is the inline object a property holds, itself or as the
// element of its slices and maps; nil when there is none.
func inlineObject(p *codegen.CodegenProperty) *codegen.CodegenProperty {
	for p != nil && (p.IsArray || p.IsMap) {
		p = p.Items
	}

	if p == nil || len(p.Vars) == 0 {
		return nil
	}

	return p
}

// refModel is the enum or union model a $ref names, "" for none.
func (g *FetchGenerator) refModel(ref string) string {
	if ref == "" {
		return ""
	}

	return g.refs[ref[strings.LastIndexByte(ref, '/')+1:]]
}

// inlineUnion is the inline oneOf/anyOf a property holds, itself or as the
// element of its slices and maps; nil when there is none.
func inlineUnion(p *codegen.CodegenProperty) *codegen.CodegenProperty {
	for p != nil && (p.IsArray || p.IsMap) {
		p = p.Items
	}

	if p == nil || p.ComposedSchemas == nil {
		return nil
	}

	return p
}

// unionDecl declares the union wrapper name for an inline oneOf/anyOf, the
// model template's union with a variant per member and the discriminator
// its variants imply.
func (g *FetchGenerator) unionDecl(name, what, description string, members *codegen.CodegenComposedSchemas, typeName func(string) string) (decls, enums []map[string]any) {
	var types []string

	for i, member := range slices.Concat(members.OneOf, members.AnyOf) {
		named := g.refModel(member.Ref)

		// An inline object member is a struct of its own, named as upstream
		// names it: <Union>OneOf, <Union>OneOf1, ...
		if obj := inlineObject(member); obj != nil {
			kind := "OneOf"
			if i >= len(members.OneOf) {
				kind, i = "AnyOf", i-len(members.OneOf)
			}

			if i > 0 {
				kind += strconv.Itoa(i)
			}

			named = typeName(name + kind)
			structs, nested := g.inlineStructs(named, "a variant of "+name, obj.Description, obj.Vars, typeName)
			decls = append(decls, structs...)
			enums = append(enums, nested...)

			g.required[named] = requiredKeys(obj.Vars)
			g.pinned[named] = pinned(named, obj.Vars)
		}

		types = append(types, GoType(member, named))
	}

	variants, _ := g.variants(types)

	union := map[string]any{
		"name": name, "union": true, "classname": name,
		"vendorExtensions": map[string]any{
			"x-go-doc":           docLines(name+" is "+what+", one of its variants as raw JSON:\nthe As methods decode it, the From methods set it.", description),
			"x-go-variants":      variants,
			"x-go-discriminator": impliedDiscriminator(variants),
		},
	}

	return append([]map[string]any{union}, decls...), enums
}

// requiredKeys is the quoted names of the required vars, comma-separated.
func requiredKeys(vars []*codegen.CodegenProperty) string {
	var required []string

	for _, v := range vars {
		if v.Required {
			required = append(required, strconv.Quote(v.BaseName))
		}
	}

	return strings.Join(required, ", ")
}

// pinned lists the properties of a struct an enum of one value fixes, a
// pydantic Literal or an OpenAPI 3.1 const: a union's From sets them, its
// As refuses a value with another. Fields are named as structFields does.
func pinned(class string, vars []*codegen.CodegenProperty) []map[string]any {
	var out []map[string]any

	names := make(map[string]int)

	for _, v := range vars {
		field := uniqueName(names, v.Name)

		values, _ := v.AllowableValues["enumVars"].([]map[string]any)
		if !v.IsEnum || len(values) != 1 || !v.Required || v.IsNullable || v.IsReadOnly || v.IsArray || v.IsMap {
			continue
		}

		decl := enumDecl("", EnumBaseType(v.AllowableValues, v.IsBoolean), v.AllowableValues, nil)
		literal, _ := decl["values"].([]map[string]any)[0]["literal"].(string)

		out = append(out, map[string]any{
			"field": field, "key": v.BaseName, "literal": literal,
			"mismatch": strconv.Quote("not a " + class + ": " + strings.ReplaceAll(v.BaseName, "%", "%%") + " is %v"),
		})
	}

	return out
}

// impliedDiscriminator is the discriminator of a union whose every variant
// pins one string property to a value of its own, false when there is none.
func impliedDiscriminator(variants []map[string]any) any {
	if len(variants) == 0 {
		return false
	}

	first, _ := variants[0]["pinned"].([]map[string]any)

	for _, candidate := range first {
		key := candidate["key"]
		values := make(map[string]bool)

		var mappings []map[string]any

		for _, variant := range variants {
			pins, _ := variant["pinned"].([]map[string]any)

			i := slices.IndexFunc(pins, func(p map[string]any) bool { return p["key"] == key })
			if i < 0 {
				break
			}

			literal, _ := pins[i]["literal"].(string)
			if values[literal] || !strings.HasPrefix(literal, `"`) {
				break
			}

			values[literal] = true
			mappings = append(mappings, map[string]any{"value": literal, "type": variant["type"]})
		}

		if len(mappings) == len(variants) {
			name, _ := key.(string)

			return map[string]any{"name": name, "quoted": strconv.Quote(name), "tag": jsonTag(name), "mappings": mappings}
		}
	}

	return false
}

// inlineStructs declares the struct name for an inline object's properties,
// and the inline enums and objects those hold. what says where the object
// sits, for the doc comment.
func (g *FetchGenerator) inlineStructs(name, what, description string, vars []*codegen.CodegenProperty, typeName func(string) string) (decls, enums []map[string]any) {
	ext := map[string]any{}
	fields := g.structFields(&codegen.CodegenModel{Classname: name, AllVars: vars}, ext, typeName)

	nested, _ := ext["x-go-inline"].([]map[string]any)
	enums, _ = ext["x-go-enums"].([]map[string]any)

	decl := map[string]any{"name": name, "union": false, "doc": docLines(name+" is "+what+".", description), "fields": fields}

	return append([]map[string]any{decl}, nested...), enums
}

// modelTypeName names an inline type: base, with a count when a type has
// the name already (Foo + BarBaz and FooBar + Baz run together).
func (g *FetchGenerator) modelTypeName(base string) string {
	name := base
	for i := 2; g.types[name]; i++ {
		name = base + strconv.Itoa(i)
	}

	g.types[name] = true

	return name
}

// opTypeNamer names the inline types of operation op as modelTypeName does,
// the same name each time it is asked again: an operation under several tags
// is processed once per tag.
func (g *FetchGenerator) opTypeNamer(op string) func(string) string {
	return func(base string) string {
		key := op + "\x00" + base
		if name, ok := g.opTypes[key]; ok {
			return name
		}

		name := g.modelTypeName(base)
		g.opTypes[key] = name

		return name
	}
}

// uniqueName suffixes a name already taken in a struct with a count.
func uniqueName(taken map[string]int, name string) string {
	taken[name]++
	if n := taken[name]; n > 1 {
		for taken[name+strconv.Itoa(n)] > 0 {
			n++
		}

		name += strconv.Itoa(n)
		taken[name]++
	}

	return name
}

// jsonTag is the struct tag for a JSON name and options, as a raw string
// unless the name needs escaping.
func jsonTag(nameOpts string) string {
	tag := "json:" + strconv.Quote(nameOpts)
	if strings.ContainsAny(tag, "`") {
		return strconv.Quote(tag)
	}

	return "`" + tag + "`"
}

// enumDecl is a named enum type with one constant per value. owner is the
// property an inline enum belongs to, nil for an enum model.
func enumDecl(name, base string, allowable map[string]any, owner *codegen.CodegenProperty) map[string]any {
	vars, _ := allowable["enumVars"].([]map[string]any)
	unescape := strings.NewReplacer(`\\`, `\`, `\'`, `'`)

	values := make([]map[string]any, 0, len(vars))
	for _, v := range vars {
		literal, _ := v["value"].(string)
		if base == "string" {
			literal = strconv.Quote(unescape.Replace(literal))
		}

		member, _ := v["name"].(string)
		values = append(values, map[string]any{
			"name":    name + exported(member, ""),
			"type":    name,
			"literal": literal,
		})
	}

	var doc []string
	if owner != nil {
		doc = []string{"// " + name + " is a value of " + owner.BaseName + "."}
	}

	return map[string]any{"name": name, "base": base, "values": values, "doc": doc}
}

// breakCycles turns into a pointer each struct-typed field held by value
// (required, or in an Opt) that closes a cycle of such fields (A holds a B
// holds an A), which Go cannot lay out. Models are walked in name order, so
// the choice is stable.
func breakCycles(fields map[string][]map[string]any, kinds map[string]string) {
	state := make(map[string]int) // 1 on the path, 2 done

	var visit func(name string)

	visit = func(name string) {
		state[name] = 1

		for _, f := range fields[name] {
			ref, _ := f["ref"].(string)
			if kinds[ref] != "struct" {
				continue
			}

			switch state[ref] {
			case 1:
				if t, _ := f["type"].(string); strings.HasPrefix(t, "Opt[") {
					f["type"] = "Opt[*" + ref + "]"
				} else {
					f["type"] = "*" + ref
				}

			case 0:
				visit(ref)
			}
		}

		state[name] = 2
	}

	names := slices.Sorted(maps.Keys(fields))
	for _, name := range names {
		if state[name] == 0 {
			visit(name)
		}
	}
}

// unionExtensions describes a oneOf/anyOf model's variants and, with a
// discriminator, how its values map to models.
func (g *FetchGenerator) unionExtensions(m *codegen.CodegenModel, ext map[string]any, bySchema map[string]*codegen.CodegenModel) {
	var types []string

	types = append(types, m.OneOfModels...)
	for _, item := range m.OneOfArrays {
		types = append(types, "[]"+item)
	}

	for _, p := range m.OneOfPrimitives {
		types = append(types, GoType(p, ""))
	}

	for _, name := range m.AnyOf {
		types = append(types, g.ToModelName(name))
	}

	variants, seen := g.variants(types)
	ext["x-go-variants"] = variants

	d := m.Discriminator
	if d == nil {
		ext["x-go-discriminator"] = impliedDiscriminator(variants)
		return
	}

	var mappings []map[string]any

	for _, mm := range d.MappedModels {
		mappings = append(mappings, map[string]any{"value": strconv.Quote(mm.MappingName), "type": g.ToModelName(mm.ModelName)})
	}

	// Without a mapping a value names its schema, as OpenAPI defines.
	if len(d.Mapping) == 0 {
		implicit := make(map[string]string)

		for _, member := range bySchema {
			if seen[member.Classname] {
				implicit[member.SchemaName] = member.Classname
			}
		}

		for _, value := range slices.Sorted(maps.Keys(implicit)) {
			mappings = append(mappings, map[string]any{"value": strconv.Quote(value), "type": implicit[value]})
		}
	}

	ext["x-go-discriminator"] = map[string]any{
		"name":     d.PropertyBaseName,
		"quoted":   strconv.Quote(d.PropertyBaseName),
		"tag":      jsonTag(d.PropertyBaseName),
		"mappings": mappings,
	}
}

// variants describes a union's variant per type, in order, without repeats;
// seen holds the types. A model variant's required properties tell a value
// of it from the others: As refuses JSON that lacks one.
func (g *FetchGenerator) variants(types []string) (variants []map[string]any, seen map[string]bool) {
	seen = make(map[string]bool)

	for _, t := range types {
		if seen[t] {
			continue
		}

		seen[t] = true
		variants = append(variants, map[string]any{"type": t, "method": variantName(t), "required": g.required[t], "pinned": g.pinned[t]})
	}

	return variants, seen
}

// variantName names the AsX/FromX methods of a union variant after its
// type: Cat, CatList for []Cat, StringMap for map[string]string, Time.
func variantName(t string) string {
	switch {
	case strings.HasPrefix(t, "[]"):
		return variantName(t[2:]) + "List"
	case strings.HasPrefix(t, "map[string]"):
		return variantName(t[len("map[string]"):]) + "Map"
	}

	t = t[strings.LastIndexByte(t, '.')+1:]

	return strings.ToUpper(t[:1]) + t[1:]
}

// docLines turns texts into // comment lines, a bare // between texts.
func docLines(texts ...string) []string {
	lines := []string{}

	for _, text := range texts {
		text = strings.TrimSpace(strings.ReplaceAll(text, "\r", ""))
		if text == "" {
			continue
		}

		if len(lines) > 0 {
			lines = append(lines, "//")
		}

		for _, line := range strings.Split(text, "\n") {
			if line = strings.TrimRight(line, " \t"); line == "" {
				lines = append(lines, "//")
			} else {
				lines = append(lines, "// "+line)
			}
		}
	}

	return lines
}

// clone copies vendor extensions before they are written to: the parser
// shares the spec's own map between copies of an operation.
func clone(ext map[string]any) map[string]any {
	if ext == nil {
		return map[string]any{}
	}

	return maps.Clone(ext)
}

// PostProcessOperations names each operation's methods and Params type and
// types its parameters, body, responses and auth for the api template.
func (g *FetchGenerator) PostProcessOperations(ops []*codegen.CodegenOperation) []*codegen.CodegenOperation {
	names := make(map[string]bool, len(ops))
	for _, op := range ops {
		names[g.SanitizeOperationId(op.OperationId)] = true
	}

	for _, op := range ops {
		base := g.SanitizeOperationId(op.OperationId)

		// FooRaw and FooRequest are Foo's own methods.
		name := base
		for _, suffix := range []string{"Raw", "Request"} {
			if s, ok := strings.CutSuffix(base, suffix); ok && names[s] {
				name = base + "Method"
			}
		}

		op.Nickname = name

		ext := clone(op.VendorExtensions)
		ext["x-go-name"] = name
		ext["x-go-params"] = g.opTypeNamer(op.OperationId)(base + "Params")
		ext["x-go-has-params"] = len(op.AllParams) > 0
		ext["x-go-doc"] = docLines(name+" sends "+op.HttpMethod+" "+op.Path+".", op.Summary, op.Notes)
		ext["x-go-path"] = strconv.Quote(op.Path)
		ext["x-go-method"] = strconv.Quote(op.HttpMethod)
		ext["x-go-auth"] = authEntries(op)

		ext["x-go-inline"], ext["x-go-inline-enums"] = []map[string]any{}, []map[string]any{}
		g.paramExtensions(op, ext, base)
		g.responseExtensions(op, ext, base)

		op.VendorExtensions = ext
	}

	return ops
}

// paramExtensions declares the Params fields and, per location, how each
// parameter is sent. An inline object parameter gets a type named after the
// operation (base) and the parameter.
func (g *FetchGenerator) paramExtensions(op *codegen.CodegenOperation, ext map[string]any, base string) {
	taken := make(map[string]int)
	fields := []map[string]any{}
	byLocation := map[string][]map[string]any{}
	files := false
	body := ""

	for _, p := range op.AllParams {
		vars, description := p.Vars, p.Description
		if obj := inlineObject(p.Items); len(vars) == 0 && obj != nil && (p.IsArray || p.IsMap) {
			vars, description = obj.Vars, obj.Description
		}

		named := ""
		if ref := g.refModel(p.Ref); g.enums[ref] {
			named = ref
		}

		typeName := g.opTypeNamer(op.OperationId)
		what := "the " + p.BaseName + " parameter of " + base

		if len(vars) > 0 {
			named = typeName(base + p.ParamName)
			g.opInline(ext)(g.inlineStructs(named, what, description, vars, typeName))
		}

		if p.ComposedSchemas != nil {
			named = typeName(base + p.ParamName)
			g.opInline(ext)(g.unionDecl(named, what, p.Description, p.ComposedSchemas, typeName))
		}

		t := GoParamType(p, named)
		if p.IsBodyParam && p.DataType == "Blob" && !p.IsModel {
			t = "io.Reader"
		}

		if !p.Required && !isContainer(t) && !g.containers[t] {
			t = "*" + t
		}

		field := uniqueName(taken, p.ParamName)
		fields = append(fields, map[string]any{"name": field, "type": t, "doc": docLines(p.Description)})

		entry := map[string]any{
			"field":   field,
			"quoted":  strconv.Quote(p.BaseName),
			"style":   paramStyle(p),
			"explode": p.IsExplode,
		}

		switch {
		case p.IsBodyParam:
			body = field
			g.bodyKind(p, t, field, ext)

		case p.IsFormParam:
			byLocation["form"] = append(byLocation["form"], entry)
			files = files || strings.HasSuffix(t, "File")

		case p.IsPathParam:
			entry["placeholder"] = strconv.Quote("{" + p.BaseName + "}")
			byLocation["path"] = append(byLocation["path"], entry)

		case p.IsQueryParam:
			byLocation["query"] = append(byLocation["query"], entry)
		case p.IsHeaderParam:
			byLocation["header"] = append(byLocation["header"], entry)
		case p.IsCookieParam:
			byLocation["cookie"] = append(byLocation["cookie"], entry)
		}
	}

	ext["x-go-fields"] = fields
	for _, loc := range []string{"path", "query", "header", "cookie", "form"} {
		ext["x-go-in-"+loc] = byLocation[loc]
	}

	// ponytail: a multipart body is built in a bytes.Buffer, files included,
	// so a request holds its uploads in memory; the upgrade is an io.Pipe
	// that a goroutine writes once the request is sent.
	// A form with a file goes as multipart whatever media type it declares,
	// as upstream's Go client does: only multipart carries a file.
	form := len(byLocation["form"]) > 0
	multipart := op.IsMultipart || files
	ext["x-go-has-body"] = body != "" || form
	ext["x-go-body-multipart"] = form && multipart
	ext["x-go-body-form"] = form && !multipart
}

// bodyKind sets how a body parameter is encoded: as it is for a reader, as
// text for a string the content type does not mark as JSON, else as JSON.
func (g *FetchGenerator) bodyKind(p *codegen.CodegenParameter, t, field string, ext map[string]any) {
	ext["x-go-content-type"] = strconv.Quote(p.ContentType)

	switch {
	case t == "io.Reader":
		ext["x-go-body-reader"] = field
	case strings.TrimPrefix(t, "*") == "string" && !isJSONMime(p.ContentType):
		ext["x-go-body-text"] = field
	default:
		ext["x-go-body-json"] = field
	}
}

// opInline returns a func adding an operation's inline type declarations and
// enums to ext, for the API file to declare next to the Params type.
func (g *FetchGenerator) opInline(ext map[string]any) func(decls, enums []map[string]any) {
	return func(decls, enums []map[string]any) {
		declared, _ := ext["x-go-inline"].([]map[string]any)
		ext["x-go-inline"] = append(declared, decls...)

		declaredEnums, _ := ext["x-go-inline-enums"].([]map[string]any)
		ext["x-go-inline-enums"] = append(declaredEnums, enums...)
	}
}

// isJSONMime reports whether a media type is JSON, application/json or a
// +json suffix, as the runtime's check.
func isJSONMime(mediaType string) bool {
	mt, _, _ := strings.Cut(strings.ToLower(mediaType), ";")
	mt = strings.TrimSpace(mt)

	return mt == "" || mt == "application/json" || strings.HasSuffix(mt, "+json")
}

// paramStyle is a parameter's OpenAPI style with the default of its
// location filled in.
func paramStyle(p *codegen.CodegenParameter) string {
	switch {
	case p.IsDeepObject:
		return "deepObject"
	case p.Style != "":
		return p.Style
	case p.IsPathParam || p.IsHeaderParam:
		return "simple"
	}

	return "form"
}

// responseType is the Go type of a response body, "" for none.
func responseType(r *codegen.CodegenResponse) string {
	switch {
	case r.IsBinary || r.IsFile:
		return "[]byte"
	case r.DataType == "":
		return ""
	case r.ReturnProperty == nil:
		return r.DataType // a $ref, which the parser names but leaves untyped
	}

	return GoType(r.ReturnProperty, "")
}

// responseExtensions types the operation's result and the model each error
// status decodes into. A result that a 2xx status sends without a body is a
// pointer, nil for that status, unless it is nil-able already or a string,
// whose empty text says the same. An inline object result gets a type named
// after the operation (base).
func (g *FetchGenerator) responseExtensions(op *codegen.CodegenOperation, ext map[string]any, base string) {
	t := ""

	switch {
	case op.IsResponseBinary:
		t = "[]byte"
		ext["x-go-decode-bytes"] = true

	case op.ReturnType == "":
		if !g.streamExtensions(op, ext, base) {
			ext["x-go-discard"] = true
		}

	case op.ReturnProperty == nil:
		t = op.ReturnType
		if g.nullable[t] && !g.containers[t] {
			t = "*" + t
		}

	default:
		t = g.resultType(op, op.ReturnProperty, base+"Response", "the response of "+base, ext)
	}

	if t != "" && t != "string" && !isContainer(t) && !g.containers[t] {
		var empty []string

		for _, r := range op.Responses {
			if r.Is2xx && responseType(r) == "" {
				empty = append(empty, r.Code)
			}
		}

		if len(empty) > 0 {
			t = "*" + strings.TrimPrefix(t, "*")
			doc, _ := ext["x-go-doc"].([]string)
			ext["x-go-doc"] = append(doc, "//", "// The result is nil for a "+strings.Join(empty, " or ")+" response, which has no body.")
		}
	}

	ext["x-go-type"] = t
	if t != "" && !op.IsResponseBinary {
		ext["x-go-decode-json"] = t
		g.other2xx(op, ext, base)
	}

	var codes, ranges []map[string]any

	var def map[string]any

	for _, r := range op.Responses {
		rt := responseType(r)
		if rt == "" || rt == "[]byte" || r.Is2xx {
			continue
		}

		keys := g.modelKeys(rt)

		switch _, err := strconv.Atoi(r.Code); {
		case r.IsDefault:
			def = map[string]any{"type": rt, "keys": keys}
		case len(r.Code) == 3 && strings.EqualFold(r.Code[1:], "XX"):
			ranges = append(ranges, map[string]any{"digit": r.Code[:1], "type": rt, "keys": keys})
		case err == nil:
			codes = append(codes, map[string]any{"code": r.Code, "type": rt, "keys": keys})
		}
	}

	if len(codes) > 0 || len(ranges) > 0 || def != nil {
		ext["x-go-errors"] = map[string]any{
			"codes": codes, "hasCodes": len(codes) > 0,
			"ranges": ranges, "hasRanges": len(ranges) > 0,
			"default": def,
		}
	}
}

// modelKeys is the Go []string of the required properties of model t, the
// keys a body must hold to be decoded as one; "nil" for none.
func (g *FetchGenerator) modelKeys(t string) string {
	if required := g.required[t]; required != "" {
		return "[]string{" + required + "}"
	}

	return "nil"
}

// resultType is the Go type of a result or a streamed item, prop: an
// inline object or union is a type named name, declared for the operation;
// a nullable one is a pointer.
func (g *FetchGenerator) resultType(op *codegen.CodegenOperation, prop *codegen.CodegenProperty, name, what string, ext map[string]any) string {
	named := ""

	typeName := g.opTypeNamer(op.OperationId)

	if obj := inlineObject(prop); obj != nil {
		named = typeName(name)
		g.opInline(ext)(g.inlineStructs(named, what, obj.Description, obj.Vars, typeName))
	}

	if u := inlineUnion(prop); u != nil {
		named = typeName(name)
		g.opInline(ext)(g.unionDecl(named, what, u.Description, u.ComposedSchemas, typeName))
	}

	t := GoType(prop, named)
	if prop.IsNullable && t != "string" && !isContainer(t) && !g.containers[t] {
		t = "*" + t
	}

	return t
}

// streamExtensions types an operation whose 2xx response streams its items
// (x-stream, see the parser's streamItem) and reports whether it does: JSON
// Lines as an iter.Seq2 of the item type, server-sent events as one of
// ServerSentEvent of it. Another streamed media type (json-seq, multipart)
// has only its Raw method.
func (g *FetchGenerator) streamExtensions(op *codegen.CodegenOperation, ext map[string]any, base string) bool {
	for _, r := range op.Responses {
		item, _ := r.VendorExtensions["x-stream-item"].(*codegen.CodegenProperty)
		mediaType, _ := r.VendorExtensions["x-stream"].(string)
		mediaType, _, _ = strings.Cut(mediaType, ";")

		decode := map[string]string{"application/jsonl": "decodeStream", "application/x-ndjson": "decodeStream", "text/event-stream": "decodeEvents"}[mediaType]
		if !r.Is2xx || item == nil || decode == "" {
			continue
		}

		t := g.resultType(op, item, base+"Item", "an item of "+base, ext)

		yields := t
		if decode == "decodeEvents" {
			yields = "ServerSentEvent[" + t + "]"
		}

		ext["x-go-stream"] = map[string]any{"item": yields, "decode": decode + "[" + t + "]"}

		doc, _ := ext["x-go-doc"].([]string)
		ext["x-go-doc"] = append(doc, "//", "// Range the result to its end, break or cancel ctx: until then the response stays open.")

		return true
	}

	return false
}

// other2xx lists the 2xx statuses whose body is of another type than the
// result (a 201 Order next to a 202 Problem): the typed method returns
// them as a *ResponseError holding that body, which the result cannot. An
// inline object body gets a type named after the operation and status.
func (g *FetchGenerator) other2xx(op *codegen.CodegenOperation, ext map[string]any, base string) {
	result, _ := ext["x-go-type"].(string)
	if result == "any" {
		return // any holds every body
	}

	var codes []map[string]any

	for _, r := range op.Responses {
		// An inline body of the result's shape is the result (a 200 and 201
		// of one schema); the parser gives each its own property.
		if _, err := strconv.Atoi(r.Code); err != nil || !r.Is2xx || r.DataType == "" || r.ReturnProperty != nil && reflect.DeepEqual(r.ReturnProperty, op.ReturnProperty) {
			continue
		}

		rt := responseType(r)
		if obj := inlineObject(r.ReturnProperty); obj != nil {
			typeName := g.opTypeNamer(op.OperationId)
			named := typeName(base + r.Code + "Response")
			g.opInline(ext)(g.inlineStructs(named, "the "+r.Code+" response of "+base, obj.Description, obj.Vars, typeName))
			rt = GoType(r.ReturnProperty, named)
		}

		if rt == "" || rt == "any" || rt == "[]byte" || rt == strings.TrimPrefix(result, "*") {
			continue
		}

		codes = append(codes, map[string]any{"code": r.Code, "type": rt, "keys": g.modelKeys(rt)})

		doc, _ := ext["x-go-doc"].([]string)
		ext["x-go-doc"] = append(doc, "//", "// A "+r.Code+" response is a *ResponseError with its "+rt+" body in Model.")
	}

	if len(codes) > 0 {
		ext["x-go-other-2xx"] = map[string]any{"codes": codes}
	}
}

// authEntries lists how each security scheme of the operation is sent.
func authEntries(op *codegen.CodegenOperation) []map[string]any {
	opName := op.OperationIdOriginal
	if opName == "" {
		opName = op.OperationId
	}

	entries := []map[string]any{}

	for _, a := range op.AuthMethods {
		e := map[string]any{
			"op": strconv.Quote(opName), "scheme": strconv.Quote(a.Name),
			"apiKey": false, "basic": false, "bearer": false,
		}

		switch {
		case a.IsApiKey:
			in := "header"
			if a.IsKeyInQuery {
				in = "query"
			} else if a.IsKeyInCookie {
				in = "cookie"
			}

			e["apiKey"], e["key"], e["in"] = true, strconv.Quote(a.KeyParamName), in

		case a.IsBasicBasic:
			e["basic"] = true
		case a.IsBasicBearer || a.IsOAuth || a.IsOpenId:
			scopes := "nil"

			if len(a.Scopes) > 0 {
				quoted := make([]string, len(a.Scopes))
				for i, s := range a.Scopes {
					quoted[i] = strconv.Quote(fmt.Sprint(s["scope"]))
				}

				scopes = "[]string{" + strings.Join(quoted, ", ") + "}"
			}

			e["bearer"], e["scopes"] = true, scopes

		default:
			continue
		}

		entries = append(entries, e)
	}

	return entries
}
