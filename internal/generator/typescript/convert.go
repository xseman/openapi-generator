package typescript

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/xseman/openapi-generator/internal/codegen"
)

// converter writes the TypeScript expressions that turn a value from its
// JSON into the client's type (FromJSON) or back (ToJSON). It descends into
// arrays and maps at any depth, so a list of lists of models, a map of
// dates or a map of lists of models convert as a model or a date does.
type converter struct {
	g *FetchGenerator

	// to converts into JSON; else from it.
	to bool

	// rt prefixes the runtime's helpers: "" in a model file, which imports
	// them, "runtime." in an API file.
	rt string

	// used collects the models whose functions the expressions call.
	used map[string]bool
}

// expr returns the expression converting x, a value of p; x itself when
// JSON holds the value as the client does.
func (c *converter) expr(p *codegen.CodegenProperty, x string) string {
	if p == nil || c.g.WithoutRuntimeChecks {
		return x
	}

	if p.IsArray || p.IsMap {
		item := c.expr(p.Items, "v")
		if item == "v" {
			return x
		}

		if p.Items.IsNullable && (p.Items.IsDateType || p.Items.IsDateTimeType) {
			item = "v == null ? v : " + item
		}

		fn := "(v: any) => " + item
		if name, ok := strings.CutSuffix(item, "(v)"); ok && identifier.MatchString(name) {
			fn = name
		}

		if p.IsArray {
			return "(" + x + " as Array<any>).map(" + fn + ")"
		}

		return c.rt + "mapValues(" + x + ", " + fn + ")"
	}

	switch {
	case p.IsDateType && c.to:
		return c.rt + "serializeDate(" + x + ")"
	case p.IsDateType:
		return c.rt + "parseDate(" + x + ")"
	case p.IsDateTimeType && c.to:
		return c.rt + "serializeDateTime(" + x + ")"
	case p.IsDateTimeType:
		return c.rt + "parseDateTime(" + x + ")"
	}

	model := c.g.modelOf(p)
	if model == "" {
		return c.union(p, x)
	}

	c.used[model] = true

	return model + c.fn() + "(" + x + ")"
}

// fn is the suffix of the models' functions the converter calls.
func (c *converter) fn() string {
	if c.to {
		return "ToJSON"
	}

	return "FromJSON"
}

var identifier = regexp.MustCompile(`^[\w$.]+$`)

// union converts x, a value of an inline union: of models by the model its
// discriminator names, a property every member pins to a string of its own
// (pydantic's Literal), else by the model it fits best;
// of scalars, a date among them, by the date's text. A union with another
// member, an array say, stays as it is.
func (c *converter) union(p *codegen.CodegenProperty, x string) string {
	if p.ComposedSchemas == nil {
		return x
	}

	members := slices.Concat(p.ComposedSchemas.OneOf, p.ComposedSchemas.AnyOf)

	dates := slices.DeleteFunc(slices.Clone(members), func(m *codegen.CodegenProperty) bool { return !m.IsDateType && !m.IsDateTimeType })
	if len(dates) == 1 {
		pattern := `/^\d{4}-\d{2}-\d{2}$/`
		if dates[0].IsDateTimeType {
			pattern = `/^\d{4}-\d{2}-\d{2}T/`
		}

		if c.to {
			return "(" + x + " instanceof Date ? " + c.expr(dates[0], x) + " : " + x + ")"
		}

		return "(typeof " + x + ` === "string" && ` + pattern + ".test(" + x + ") ? " + c.expr(dates[0], x) + " : " + x + ")"
	}

	if d := c.discriminated(members, x); d != "" {
		return d
	}

	return c.bestFit(members, x)
}

// discriminated converts x by a property every member of the union pins to
// a string of its own, or is "".
func (c *converter) discriminated(members []*codegen.CodegenProperty, x string) string {
	var (
		key   string
		cases []string
	)

	used := map[string]bool{}

	for _, m := range members {
		model := c.g.byName[c.g.modelOf(m)]
		if model == nil {
			return ""
		}

		pin := pinned(model, key, c.to)
		if pin == nil {
			return ""
		}

		key = pin.key

		value := strconv.Quote(fmt.Sprint(pin.value))

		if slices.ContainsFunc(cases, func(s string) bool { return strings.HasPrefix(s, value+":") }) {
			return ""
		}

		used[model.Classname] = true
		cases = append(cases, value+": "+model.Classname+c.fn())
	}

	if len(cases) == 0 {
		return ""
	}

	maps.Copy(c.used, used)

	return c.rt + "discriminate(" + x + ", " + strconv.Quote(key) + ", { " + strings.Join(cases, ", ") + " })"
}

// bestFit converts x by the member model it fits best (the runtime's
// bestFit): one whose required keys it has, declaring the most of its keys
// and the fewest it does not. A scalar member (Stripe's id next to the
// object it expands to) takes a value that is no object.
func (c *converter) bestFit(members []*codegen.CodegenProperty, x string) string {
	var models [][]*codegen.CodegenModel

	for _, m := range members {
		model := c.g.byName[c.g.modelOf(m)]

		switch {
		case model == nil && m.IsPrimitiveType && !m.IsContainer:
			continue
		case model == nil:
			return x
		}

		models = append(models, []*codegen.CodegenModel{model})
	}

	if len(models) == 0 {
		return x
	}

	return c.rt + "bestFit(" + x + ", " + c.fits(models) + ") ?? " + x
}

// fits lists members for the runtime's bestFit: each one's required keys,
// its keys and its converter, in JSON (from) or in TypeScript (to). A
// member is a model, or the intersection of several (an allOf), converted
// by each and merged.
func (c *converter) fits(members [][]*codegen.CodegenModel) string {
	cases := make([]string, 0, len(members))

	for _, parts := range members {
		var required, known, fns []string

		for _, m := range parts {
			for _, v := range c.keyVars(m) {
				name := strconv.Quote(v.BaseName)
				if c.to {
					name = strconv.Quote(v.Name)
				}

				if !slices.Contains(known, name) {
					known = append(known, name)
				}

				if v.Required && !slices.Contains(required, name) {
					required = append(required, name)
				}
			}

			c.used[m.Classname] = true
			fns = append(fns, m.Classname+c.fn())
		}

		fn := fns[0]
		if len(fns) > 1 {
			fn = "(v: any) => ({ ..." + strings.Join(fns, "(v), ...") + "(v) })"
		}

		cases = append(cases, "[["+strings.Join(required, ", ")+"], ["+strings.Join(known, ", ")+"], "+fn+"]")
	}

	return "[" + strings.Join(cases, ", ") + "]"
}

// keyVars are the properties whose keys tell a value of m: its own and
// inherited ones, or for a union model those of its members, none required.
func (c *converter) keyVars(m *codegen.CodegenModel) []*codegen.CodegenProperty {
	if !m.HasOneOf {
		if len(m.AllVars) > 0 {
			return m.AllVars
		}

		return m.Vars
	}

	var vars []*codegen.CodegenProperty

	for _, name := range m.OneOfModels {
		if member := c.g.byName[name]; member != nil && !member.HasOneOf {
			for _, v := range member.Vars {
				vars = append(vars, &codegen.CodegenProperty{Name: v.Name, BaseName: v.BaseName})
			}
		}
	}

	return vars
}

type pin struct {
	key   string
	value any
}

// pinned finds the property of m that holds one string value, named key
// in JSON (to false) or in TypeScript (to true) when key is not "".
func pinned(m *codegen.CodegenModel, key string, to bool) *pin {
	for _, v := range m.Vars {
		values, _ := v.AllowableValues["values"].([]any)
		if !v.IsEnum || !v.Required || v.IsNullable || len(values) != 1 {
			continue
		}

		if _, ok := values[0].(string); !ok {
			continue
		}

		name := v.BaseName
		if to {
			name = v.Name
		}

		if key == "" || key == name {
			return &pin{name, values[0]}
		}
	}

	return nil
}

// modelOf names the generated model whose FromJSON and ToJSON convert a
// value of p, or "". A $ref to a union is typed as its members joined,
// "Cat | Dog", but converts with the union model's functions.
func (g *FetchGenerator) modelOf(p *codegen.CodegenProperty) string {
	if p.Ref != "" {
		name := g.ToModelName(p.Ref[strings.LastIndex(p.Ref, "/")+1:])
		if m := g.byName[name]; m != nil && m.HasOneOf {
			return name
		}
	}

	if p.IsModel && g.byName[p.Datatype] != nil {
		return p.Datatype
	}

	return ""
}

// varFromJSON is the value of model property p in FromJSON: its JSON
// converted, absent or null as the property allows.
func (c *converter) varFromJSON(p *codegen.CodegenProperty) string {
	x := "json[" + strconv.Quote(p.BaseName) + "]"
	if objectMembers[p.BaseName] {
		x = "(Object.prototype.hasOwnProperty.call(json, " + strconv.Quote(p.BaseName) + ") ? " + x + " : undefined)"
	}

	v := c.expr(p, x)

	switch {
	case !p.Required && p.IsNullable:
		return x + " === undefined ? undefined : " + x + " === null ? null : " + v
	case !p.Required:
		return x + " == null ? undefined : " + v
	case p.IsNullable:
		return x + " == null ? null : " + v
	case v != x && (p.IsContainer || p.IsDateType || p.IsDateTimeType):
		// A required value the server left out stays as it came.
		return x + " == null ? " + x + " : " + v
	}

	return v
}

// objectMembers are the keys every object inherits: JSON without one of
// them still reads it, so a property of that name reads its own key only.
var objectMembers = map[string]bool{
	"__proto__": true, "constructor": true, "hasOwnProperty": true, "isPrototypeOf": true,
	"propertyIsEnumerable": true, "toLocaleString": true, "toString": true, "valueOf": true,
	"__defineGetter__": true, "__defineSetter__": true, "__lookupGetter__": true, "__lookupSetter__": true,
}

// varToJSON is the JSON of model property p in ToJSON. A null stays null
// where the property takes one; an optional property left out stays out.
func (c *converter) varToJSON(p *codegen.CodegenProperty) string {
	x := "value[" + strconv.Quote(p.Name) + "]"

	v := c.expr(p, x)
	if v == x || c.g.modelOf(p) != "" && !p.IsContainer {
		return v
	}

	if !p.Required && !p.IsNullable {
		return x + " == null ? undefined : " + v
	}

	return x + " == null ? " + x + " : " + v
}

// asProperty is parameter p as a property, for the converter.
func asProperty(p *codegen.CodegenParameter) *codegen.CodegenProperty {
	return &codegen.CodegenProperty{
		Datatype: p.DataType, IsModel: p.IsModel, IsArray: p.IsArray, IsMap: p.IsMap, IsContainer: p.IsContainer,
		Items: p.Items, IsDateType: p.IsDateType, IsDateTimeType: p.IsDateTimeType, Ref: p.Ref,
	}
}
