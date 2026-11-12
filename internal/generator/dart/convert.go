package dart

import (
	"strings"

	"github.com/xseman/openapi-generator/internal/codegen"
)

// modelOf is the class of the model p is: the one its $ref names, or the
// one the parser typed it as; "" for a value that is no model.
func (g *FetchGenerator) modelOf(p *codegen.CodegenProperty) string {
	if p == nil {
		return ""
	}

	if model := g.refs[p.Ref[strings.LastIndexByte(p.Ref, '/')+1:]]; model != "" {
		return model
	}

	if p.IsModel && g.kinds[p.DataType] != "" {
		return p.DataType
	}

	return ""
}

// typeOf is the Dart type of p; enum is the enum or the union declared for
// what its leaf lists inline (see inline), "" for none.
func (g *FetchGenerator) typeOf(p *codegen.CodegenProperty, enum string) string {
	if model := g.modelOf(leaf(p)); model != "" {
		return DartType(p, model)
	}

	return DartType(p, enum)
}

// fromJSON returns the expression that reads x, a decoded JSON value that
// is not null, as a value of p: lists and maps at any depth, a model by its
// fromJson, an alias as the type it stands for.
func (g *FetchGenerator) fromJSON(p *codegen.CodegenProperty, x, enum string) string {
	switch {
	case p == nil:
		return x
	case p.IsArray:
		return "(" + x + " as List<Object?>).map((e) => " + g.itemFromJSON(p.Items, "e", enum) + ").toList()"
	case p.IsMap:
		return "(" + x + " as Map<String, Object?>).map((k, v) => MapEntry(k, " + g.itemFromJSON(p.Items, "v", enum) + "))"
	}

	model := g.modelOf(p)

	switch {
	case g.kinds[model] == "alias":
		return g.fromJSON(g.aliases[model], x, "")
	case g.kinds[model] == "struct":
		return model + ".fromJson(" + x + " as Map<String, Object?>)"
	case model != "":
		return model + ".fromJson(" + x + ")"
	case enum != "" && len(p.Vars) > 0:
		return enum + ".fromJson(" + x + " as Map<String, Object?>)"
	case enum != "" && (p.IsEnum || len(inlineUnion(p)) > 0):
		return enum + ".fromJson(" + x + ")"
	}

	switch t := DartType(p, ""); t {
	case "Object?":
		return x
	case "DateTime":
		return "DateTime.parse(" + x + " as String)"
	case "int":
		return "(" + x + " as num).toInt()"
	case "double":
		return "(" + x + " as num).toDouble()"
	default:
		return x + " as " + t
	}
}

func (g *FetchGenerator) itemFromJSON(p *codegen.CodegenProperty, x, enum string) string {
	expr := g.fromJSON(p, x, enum)
	if p != nil && p.IsNullable {
		return orNull(x, expr)
	}

	return expr
}

// toJSON returns the expression that writes x, a value of p that is not
// null, as JSON; x itself when JSON holds the value as the client does.
func (g *FetchGenerator) toJSON(p *codegen.CodegenProperty, x, enum string) string {
	switch {
	case p == nil:
		return x
	case p.IsArray:
		item := g.itemToJSON(p.Items, "e", enum)
		if item == "e" {
			return x
		}

		return x + ".map((e) => " + item + ").toList()"

	case p.IsMap:
		item := g.itemToJSON(p.Items, "v", enum)
		if item == "v" {
			return x
		}

		return x + ".map((k, v) => MapEntry(k, " + item + "))"
	}

	model := g.modelOf(p)

	switch {
	case g.kinds[model] == "alias":
		return g.toJSON(g.aliases[model], x, "")
	case model != "", enum != "" && (p.IsEnum || len(p.Vars) > 0 || len(inlineUnion(p)) > 0):
		return x + ".toJson()"
	case p.IsDate:
		return "formatDate(" + x + ")"
	case p.IsDateTime:
		return x + ".toUtc().toIso8601String()"
	}

	return x
}

func (g *FetchGenerator) itemToJSON(p *codegen.CodegenProperty, x, enum string) string {
	expr := g.toJSON(p, x, enum)
	if p != nil && p.IsNullable {
		return orNull(x, expr)
	}

	return expr
}
