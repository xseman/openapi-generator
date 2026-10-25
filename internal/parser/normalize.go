package parser

import (
	"fmt"
	"mime"
	"slices"

	"github.com/getkin/kin-openapi/openapi3"
)

// normalize applies two of the upstream OpenAPINormalizer's default rules to
// every schema of the loaded document, so an OpenAPI 3.1 spec reads as the
// 3.0 shapes the parser knows:
//
//   - NORMALIZE_31SPEC: `type: [T, "null"]` is T with nullable, `const: v`
//     an enum of v, a null among enum values nullable, and a string of
//     contentMediaType application/octet-stream a binary string.
//   - SIMPLIFY_ONEOF_ANYOF: a `{type: "null"}` member of oneOf/anyOf makes
//     the schema nullable instead; when one member is left after that, it is
//     the schema (a $ref one as `allOf: [$ref]`, the 3.0 spelling of a
//     nullable $ref). Upstream also unwraps a lone member without a null.
//   - SIMPLIFY_ONEOF_ANYOF_ENUM: a oneOf/anyOf of enums of one type is one
//     enum of all their values (a union of pydantic Literals).
//
// It runs after the property order is recorded and keeps the schema pointers
// that order is keyed on.
func (p *Parser) normalize() {
	seen := make(map[*openapi3.Schema]bool)

	walk := func(ref *openapi3.SchemaRef) { p.normalizeSchemaRef(ref, seen) }

	if c := p.Doc.Components; c != nil {
		for _, ref := range c.Schemas {
			walk(ref)
		}

		for _, param := range c.Parameters {
			normalizeParameter(param, walk)
		}

		for _, body := range c.RequestBodies {
			if body.Value != nil {
				normalizeContent(body.Value.Content, walk)
			}
		}

		for _, resp := range c.Responses {
			normalizeResponse(resp, walk)
		}
	}

	if p.Doc.Paths == nil {
		return
	}

	for _, item := range p.Doc.Paths.Map() {
		for _, param := range item.Parameters {
			normalizeParameter(param, walk)
		}

		for _, op := range item.Operations() {
			for _, param := range op.Parameters {
				normalizeParameter(param, walk)
			}

			if op.RequestBody != nil && op.RequestBody.Value != nil {
				normalizeContent(op.RequestBody.Value.Content, walk)
			}

			if op.Responses != nil {
				for _, resp := range op.Responses.Map() {
					normalizeResponse(resp, walk)
				}
			}
		}
	}
}

func normalizeParameter(param *openapi3.ParameterRef, walk func(*openapi3.SchemaRef)) {
	if param == nil || param.Value == nil {
		return
	}

	if param.Value.Schema != nil {
		walk(param.Value.Schema)
	}

	normalizeContent(param.Value.Content, walk)
}

func normalizeResponse(resp *openapi3.ResponseRef, walk func(*openapi3.SchemaRef)) {
	if resp == nil || resp.Value == nil {
		return
	}

	normalizeContent(resp.Value.Content, walk)

	for _, h := range resp.Value.Headers {
		if h.Value != nil && h.Value.Schema != nil {
			walk(h.Value.Schema)
		}
	}
}

func normalizeContent(content openapi3.Content, walk func(*openapi3.SchemaRef)) {
	for _, mt := range content {
		if mt.Schema != nil {
			walk(mt.Schema)
		}
	}
}

// normalizeSchemaRef normalizes ref in place and every schema below it, each
// once. A $ref is left as it is: its target is a component, walked as one.
func (p *Parser) normalizeSchemaRef(ref *openapi3.SchemaRef, seen map[*openapi3.Schema]bool) {
	if ref == nil || ref.Ref != "" || ref.Value == nil || seen[ref.Value] {
		return
	}

	s := ref.Value
	seen[s] = true

	if s.Type != nil && s.Type.IsMultiple() && s.Type.IncludesNull() {
		s.Type.Without(openapi3.TypeNull)
		s.Nullable = true
	}

	if s.Const != nil && len(s.Enum) == 0 {
		s.Enum, s.Const = []any{s.Const}, nil
	}

	// A null among the values (pydantic's Literal["x", None]) makes the
	// schema nullable; the value stays, as a 3.0 nullable enum lists it.
	if slices.Contains(s.Enum, nil) {
		s.Nullable = true
	}

	if (s.Type.Is(openapi3.TypeString) || s.Type.IsEmpty()) && s.Format == "" && s.ContentEncoding == "" {
		if mt, _, _ := mime.ParseMediaType(s.ContentMediaType); mt == "application/octet-stream" {
			s.Type, s.Format = &openapi3.Types{openapi3.TypeString}, "binary"
		}
	}

	s.OneOf = withoutNull(s, s.OneOf)
	s.AnyOf = withoutNull(s, s.AnyOf)

	for _, members := range []openapi3.SchemaRefs{s.AllOf, s.OneOf, s.AnyOf} {
		for _, m := range members {
			p.normalizeSchemaRef(m, seen)
		}
	}

	for _, prop := range s.Properties {
		p.normalizeSchemaRef(prop, seen)
	}

	p.normalizeSchemaRef(s.Items, seen)
	p.normalizeSchemaRef(s.AdditionalProperties.Schema, seen)
	p.normalizeSchemaRef(s.Not, seen)

	mergeEnums(s)

	// The schema is rewritten where it is, so a $ref to it (a component)
	// sees the result too; an inline member keeps its property order.
	if one := collapse(s); one != nil {
		if order, ok := p.propOrder[one]; ok {
			p.propOrder[s] = order
		}

		*s = *one
	}
}

// withoutNull drops the `type: "null"` members of a oneOf/anyOf and makes s
// nullable when there was one.
func withoutNull(s *openapi3.Schema, members openapi3.SchemaRefs) openapi3.SchemaRefs {
	kept := members[:0:0]

	for _, m := range members {
		if m.Ref == "" && m.Value != nil && m.Value.Type.Is(openapi3.TypeNull) {
			s.Nullable = true
			continue
		}

		kept = append(kept, m)
	}

	if len(kept) == len(members) {
		return members
	}

	return kept
}

// collapse returns the one member a nullable oneOf/anyOf has left, as the
// schema s stands for, or nil when s is anything more.
func collapse(s *openapi3.Schema) *openapi3.Schema {
	members := s.OneOf
	if len(s.AnyOf) > 0 {
		members = s.AnyOf
	}

	if !s.Nullable || len(members) != 1 || len(s.OneOf)+len(s.AnyOf) != 1 ||
		len(s.AllOf) > 0 || len(s.Properties) > 0 || !s.Type.IsEmpty() || s.Discriminator != nil {
		return nil
	}

	m := members[0]
	if m.Ref != "" {
		return &openapi3.Schema{
			AllOf: openapi3.SchemaRefs{m}, Nullable: true,
			Title: s.Title, Description: s.Description, Default: s.Default,
			ReadOnly: s.ReadOnly, WriteOnly: s.WriteOnly, Deprecated: s.Deprecated,
		}
	}

	if m.Value == nil {
		return nil
	}

	v := m.Value
	v.Nullable = true
	v.ReadOnly = v.ReadOnly || s.ReadOnly
	v.WriteOnly = v.WriteOnly || s.WriteOnly

	if v.Description == "" {
		v.Description = s.Description
	}

	if v.Title == "" {
		v.Title = s.Title
	}

	if v.Default == nil {
		v.Default = s.Default
	}

	return v
}

// mergeEnums turns a oneOf/anyOf whose members are all enums of the same
// type into one enum of their values.
func mergeEnums(s *openapi3.Schema) {
	members := s.OneOf
	if len(s.AnyOf) > 0 {
		members = s.AnyOf
	}

	if len(members) < 2 || len(s.OneOf)+len(s.AnyOf) != len(members) || len(s.AllOf) > 0 || s.Not != nil {
		return
	}

	var values []any

	seen := make(map[string]bool) // by %#v: an enum value may be a map, which == panics on

	for _, m := range members {
		if m.Value == nil || len(m.Value.Enum) == 0 || !slices.Equal(m.Value.Type.Slice(), members[0].Value.Type.Slice()) {
			return
		}

		for _, v := range m.Value.Enum {
			if key := fmt.Sprintf("%#v", v); !seen[key] {
				seen[key] = true

				values = append(values, v)
			}
		}
	}

	t := members[0].Value.Type.Clone()
	s.Type, s.Enum, s.OneOf, s.AnyOf = &t, values, nil, nil
}
