package dart

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/xseman/openapi-generator/internal/codegen"
)

// variant is a member of a union model: the class that wraps a value of
// it, and the property that types and converts the value.
type variant struct {
	name, typ string
	prop      *codegen.CodegenProperty

	// model is the struct model the member is, nil for another one.
	model *codegen.CodegenModel
}

// unionExtensions declares a oneOf/anyOf model's variants, a class per
// member that wraps its value, and writes the lines of the fromJson that
// tells which one a JSON value is: by its discriminator, by its JSON type,
// then the struct it fits best (bestFit in the runtime); a value that is
// none of them is the Other variant, kept as decoded.
func (g *FetchGenerator) unionExtensions(m *codegen.CodegenModel, ext map[string]any, byClass map[string]*codegen.CodegenModel) {
	variants := g.variants(m, byClass)
	// Each read follows a test that promotes json to the type: no cast.
	promoted := strings.NewReplacer(
		"json as Map<String, Object?>", "json", "(json as List<Object?>)", "json",
		"(json as num).toInt()", "json", "(json as num).toDouble()", "json.toDouble()", "json as String", "json", "json as bool", "json",
	)
	read := func(v variant) string {
		return "return " + v.name + "(" + promoted.Replace(g.fromJSON(v.prop, "json", "")) + ");"
	}

	var structs []variant

	// Scalars by how much their test tells: a date before a string, an int
	// before a double; then lists, structs by fit, and any object last.
	tests := [4][]string{}

	for _, v := range variants {
		switch t := strings.TrimSuffix(g.aliased(v.prop), "?"); {
		case v.model != nil:
			structs = append(structs, v)
		case g.kinds[g.modelOf(v.prop)] == "enum":
			tests[0] = append(tests[0], "if ("+v.typ+".values.any((v) => v.value == json)) "+read(v))
		case g.kinds[g.modelOf(v.prop)] == "union":
			tests[3] = append(tests[3], "if ("+v.typ+".fromJson(json) case final v when v is! "+v.typ+"Other) return "+v.name+"(v);")
		case t == "DateTime":
			tests[0] = append(tests[0], "if (json is String && DateTime.tryParse(json) != null) "+read(v))
		case t == "int":
			tests[0] = append(tests[0], "if (json is int) "+read(v))
		case t == "double":
			tests[1] = append(tests[1], "if (json is num) "+read(v))
		case t == "String", t == "bool":
			tests[1] = append(tests[1], "if (json is "+t+") "+read(v))
		case strings.HasPrefix(t, "List<"):
			tests[2] = append(tests[2], "if (json is List) "+read(v))
		case strings.HasPrefix(t, "Map<"):
			tests[3] = append(tests[3], "if (json is Map<String, Object?>) "+read(v))
		}
	}

	lines := []string{}

	if cases := g.discriminatorCases(m, structs, read); len(cases) > 0 {
		lines = append(lines, "if (json is Map<String, Object?>) {")
		lines = append(lines, cases...)
		lines = append(lines, "}", "")
	}

	lines = append(lines, slices.Concat(tests[0], tests[1], tests[2])...)

	if len(structs) > 0 {
		fits := make([]string, 0, len(structs))
		for _, v := range structs {
			fits = append(fits, "(["+keys(v.model, true)+"], ["+keys(v.model, false)+"])")
		}

		lines = append(lines, "", "if (json is Map<String, Object?>) {", "    switch (bestFit(json, const ["+strings.Join(fits, ", ")+"])) {")

		for i, v := range structs {
			lines = append(lines, "        case "+strconv.Itoa(i)+":", "            "+read(v))
		}

		lines = append(lines, "    }", "}", "")
	}

	lines = append(lines, tests[3]...)

	decls := make([]map[string]any, 0, len(variants))

	for _, v := range variants {
		to := g.toJSON(v.prop, "value", "")
		if tag := g.tag(m, v); tag != "" {
			to = "{" + tag + ", ..." + to + "}"
		}

		decls = append(decls, map[string]any{"name": v.name, "type": v.typ, "toJson": to})
	}

	// One blank line between the steps, none around them.
	lines = slices.CompactFunc(lines, func(a, b string) bool { return a == "" && b == "" })

	indented := make([]string, 0, len(lines))

	for i, l := range lines {
		switch {
		case l != "":
			indented = append(indented, "        "+l)
		case i > 0 && i < len(lines)-1:
			indented = append(indented, "")
		}
	}

	if doc := docLines(m.Description); len(doc) > 0 {
		ext["x-dart-doc"] = append(doc, "///")
	}

	ext["x-dart-variants"] = decls
	ext["x-dart-read"] = indented
	ext["x-dart-other"] = uniqueName(g.types, m.Classname+"Other")
}

// variants lists a union's members in order, one per type: its models,
// lists of models, primitives, then the anyOf models. A member of any type
// has no variant of its own: the Other variant holds it.
func (g *FetchGenerator) variants(m *codegen.CodegenModel, byClass map[string]*codegen.CodegenModel) []variant {
	var props []*codegen.CodegenProperty

	for _, class := range m.OneOfModels {
		props = append(props, &codegen.CodegenProperty{IsModel: true, DataType: class})
	}

	for _, class := range m.OneOfArrays {
		props = append(props, &codegen.CodegenProperty{IsArray: true, Items: &codegen.CodegenProperty{IsModel: true, DataType: class}})
	}

	props = append(props, m.OneOfPrimitives...)

	for _, schema := range m.AnyOf {
		props = append(props, &codegen.CodegenProperty{IsModel: true, DataType: g.refs[schema]})
	}

	var variants []variant

	seen := map[string]bool{}

	for _, p := range props {
		t := g.typeOf(p, "")
		if t == "Object?" || seen[t] || p.IsModel && g.kinds[p.DataType] == "" || t == m.Classname {
			continue
		}

		seen[t] = true
		v := variant{name: uniqueName(g.types, m.Classname+variantName(t)), typ: t, prop: p}

		if g.kinds[g.modelOf(p)] == "struct" {
			v.model = byClass[g.modelOf(p)]
		}

		variants = append(variants, v)
	}

	return variants
}

// aliased is the type of p with an alias model seen through: a value of
// typedef Day = DateTime tests as a DateTime.
func (g *FetchGenerator) aliased(p *codegen.CodegenProperty) string {
	if alias := g.aliases[g.modelOf(p)]; alias != nil {
		return g.aliased(alias)
	}

	return g.typeOf(p, "")
}

// variantName names a variant after its type: Cat, CatList for List<Cat>,
// StringMap for Map<String, String>, DateTime.
func variantName(t string) string {
	t = strings.TrimSuffix(t, "?")

	if item, ok := strings.CutPrefix(t, "List<"); ok {
		return variantName(strings.TrimSuffix(item, ">")) + "List"
	}

	if item, ok := strings.CutPrefix(t, "Map<String, "); ok {
		return variantName(strings.TrimSuffix(item, ">")) + "Map"
	}

	if t == "Object" {
		return "Value"
	}

	return strings.ToUpper(t[:1]) + t[1:]
}

// keys is the quoted JSON keys of a model's properties, of the required
// ones only when required is set, comma-separated.
func keys(m *codegen.CodegenModel, required bool) string {
	var out []string

	for _, v := range m.AllVars {
		if v.Required || !required {
			out = append(out, Literal(v.BaseName))
		}
	}

	return strings.Join(out, ", ")
}

// discriminatorCases writes the switch that reads a union's value by its
// discriminator: the one the spec declares, with its mapping or the
// members' schema names, else a property every struct member pins to
// values of its own (a pydantic Literal, an OpenAPI 3.1 const). No lines
// without either.
func (g *FetchGenerator) discriminatorCases(m *codegen.CodegenModel, structs []variant, read func(variant) string) []string {
	byModel := map[string]variant{}
	for _, v := range structs {
		byModel[v.model.Classname] = v
	}

	// A mapping's values are text, whatever the property holds (a const 1).
	var key, scrutinee string

	cases := []string{}
	taken := map[string]bool{}

	add := func(pattern string, v variant) {
		if !taken[pattern] {
			taken[pattern] = true
			cases = append(cases, "        case "+pattern+":", "            "+read(v))
		}
	}

	if d := m.Discriminator; d != nil {
		key = d.PropertyBaseName
		scrutinee = "'${json[" + Literal(key) + "]}'"

		for _, mapped := range d.MappedModels {
			if v, ok := byModel[g.refs[mapped.ModelName]]; ok {
				add(Literal(mapped.MappingName), v)
			}
		}

		// Without a mapping a value names its schema, as OpenAPI defines.
		if len(d.Mapping) == 0 {
			for _, v := range structs {
				add(Literal(v.model.SchemaName), v)
			}
		}
	} else if key = pinnedKey(structs); key != "" {
		scrutinee = "json[" + Literal(key) + "]"

		for _, v := range structs {
			add(strings.Join(pins(v.model, key), " || "), v)
		}
	}

	if len(cases) == 0 {
		return nil
	}

	return slices.Concat([]string{"    switch (" + scrutinee + ") {"}, cases, []string{"    }"})
}

// tag is the map entry of the discriminator for a struct member that does
// not declare the property itself, so its JSON still says which member it
// is; "" for a member that declares it, or without a discriminator.
func (g *FetchGenerator) tag(m *codegen.CodegenModel, v variant) string {
	d := m.Discriminator
	if d == nil || v.model == nil || slices.ContainsFunc(v.model.AllVars, func(p *codegen.CodegenProperty) bool { return p.BaseName == d.PropertyBaseName }) {
		return ""
	}

	for _, mapped := range d.MappedModels {
		if g.refs[mapped.ModelName] == v.model.Classname {
			return Literal(d.PropertyBaseName) + ": " + Literal(mapped.MappingName)
		}
	}

	return Literal(d.PropertyBaseName) + ": " + Literal(v.model.SchemaName)
}

// pinnedKey is the property every one of the struct members pins to values
// no other member has, "" when there is none or a single member.
func pinnedKey(structs []variant) string {
	if len(structs) < 2 {
		return ""
	}

	for _, candidate := range structs[0].model.AllVars {
		seen := map[string]bool{}
		ok := true

		for _, v := range structs {
			values := pins(v.model, candidate.BaseName)
			ok = ok && len(values) > 0

			for _, value := range values {
				ok = ok && !seen[value]
				seen[value] = true
			}
		}

		if ok {
			return candidate.BaseName
		}
	}

	return ""
}

// pins is the Dart literals of the values model m fixes its property key
// to: a required enum of strings or numbers; nil when it is not one.
func pins(m *codegen.CodegenModel, key string) []string {
	for _, v := range m.AllVars {
		values, _ := v.AllowableValues["values"].([]any)
		if v.BaseName != key || !v.IsEnum || !v.Required || v.IsNullable || v.IsArray || v.IsMap {
			continue
		}

		literals := make([]string, 0, len(values))

		for _, value := range values {
			switch value := value.(type) {
			case string:
				literals = append(literals, Literal(value))
			case float64:
				literals = append(literals, strconv.FormatFloat(value, 'f', -1, 64))
			case int, int64:
				literals = append(literals, fmt.Sprint(value))
			default:
				return nil
			}
		}

		return literals
	}

	return nil
}
