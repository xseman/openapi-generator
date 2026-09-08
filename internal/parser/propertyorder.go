package parser

import (
	"fmt"
	"os"
	"sort"

	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

// buildPropertyOrder recovers the declaration order of object properties from the raw
// OpenAPI 3.x spec source and records it in p.propOrder, keyed by resolved
// *openapi3.Schema.
//
// kin-openapi stores Schema.Properties as a plain Go map, which loses the order
// properties were declared in. Java's DefaultCodegen preserves that order (it walks a
// LinkedHashMap), and matching it removes a large, permanent source of diff noise
// against the upstream baseline. Since order can't be recovered from the already-parsed
// *openapi3.T, this walks a YAML parse of the raw document (YAML is used rather than
// encoding/json because it also accepts JSON input, so one code path covers both) in
// lockstep with the corresponding parsed schema tree.
//
// This is best-effort only: components.schemas is the sole starting point (schemas
// pulled in solely via external refs aren't covered), and any failure to parse data as
// YAML/JSON simply leaves p.propOrder unset — loudly: a warning is printed so a silent
// fallback to alphabetical order doesn't look like a supported guarantee. Callers fall
// back to alphabetical order when a schema has no recorded entry.
func (p *Parser) buildPropertyOrder(data []byte) {
	if p.Doc == nil || p.Doc.Components == nil {
		return
	}

	doc, ok := parseYAMLDocument(data)
	if !ok {
		fmt.Fprintln(os.Stderr, "warning: could not parse the spec source to recover model property declaration order; falling back to alphabetical property order")
		return
	}

	schemasNode := yamlMappingValue(yamlMappingValue(doc, "components"), "schemas")
	p.recordSchemaOrder(schemasNode)
}

// buildSwagger2PropertyOrder is buildPropertyOrder's counterpart for Swagger 2.0
// sources. Swagger 2.0 has no "components" wrapper: schemas live at the top-level
// "definitions" key instead of "components.schemas". doc3 is the *openapi3.T produced
// by converting the parsed Swagger 2.0 document (openapi2conv.ToV3): the conversion
// keeps each definition's name unchanged when it moves it into doc3.Components.Schemas
// (openapi2conv.go: `for key, schema := range ToV3Schemas(doc2.Definitions) {
// doc3.Components.Schemas[key] = schema }`), so raw "definitions" entries can be
// matched to converted schemas by name exactly as buildPropertyOrder matches
// "components.schemas" entries — walkSchemaOrder itself only ever compares a YAML node
// against an *openapi3.Schema, so it doesn't care that the schema originated from a
// converted Swagger 2.0 document rather than a native OpenAPI 3.x one.
func (p *Parser) buildSwagger2PropertyOrder(data []byte) {
	if p.Doc == nil || p.Doc.Components == nil {
		return
	}

	doc, ok := parseYAMLDocument(data)
	if !ok {
		fmt.Fprintln(os.Stderr, "warning: could not parse the spec source to recover model property declaration order; falling back to alphabetical property order")
		return
	}

	p.recordSchemaOrder(yamlMappingValue(doc, "definitions"))
}

// parseYAMLDocument parses data (YAML or JSON — YAML accepts both) and returns its root
// mapping node.
func parseYAMLDocument(data []byte) (*yaml.Node, bool) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil || len(root.Content) == 0 {
		return nil, false
	}
	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return nil, false
	}
	return doc, true
}

// recordSchemaOrder walks a top-level schema-map node (either OpenAPI 3.x
// "components.schemas" or Swagger 2.0 "definitions") and records each named schema's
// property order into p.propOrder by looking it up (by the same key) in
// p.Doc.Components.Schemas.
func (p *Parser) recordSchemaOrder(schemasNode *yaml.Node) {
	if schemasNode == nil || schemasNode.Kind != yaml.MappingNode {
		return
	}

	if p.propOrder == nil {
		p.propOrder = make(map[*openapi3.Schema][]string)
	}
	for i := 0; i+1 < len(schemasNode.Content); i += 2 {
		name := schemasNode.Content[i].Value
		ref := p.Doc.Components.Schemas[name]
		if ref == nil || ref.Value == nil {
			continue
		}
		p.walkSchemaOrder(schemasNode.Content[i+1], ref.Value)
	}
}

// walkSchemaOrder records the "properties" key order of node against schema (keyed by
// schema's pointer identity in p.propOrder), then recurses into allOf/oneOf/anyOf
// members, array items, and additionalProperties in lockstep with the already-parsed
// schema tree, so inline (non-$ref) nested schemas get their property order recorded
// too. node and schema are always derived from the same source document, so their
// structure mirrors each other; a $ref member has no inline "properties" to walk here,
// and its target is covered separately when its own named definition is visited.
func (p *Parser) walkSchemaOrder(node *yaml.Node, schema *openapi3.Schema) {
	if node == nil || node.Kind != yaml.MappingNode || schema == nil {
		return
	}

	if propsNode := yamlMappingValue(node, "properties"); propsNode != nil && propsNode.Kind == yaml.MappingNode {
		names := make([]string, 0, len(propsNode.Content)/2)
		for i := 0; i+1 < len(propsNode.Content); i += 2 {
			propName := propsNode.Content[i].Value
			names = append(names, propName)
			if propRef := schema.Properties[propName]; propRef != nil && propRef.Value != nil {
				p.walkSchemaOrder(propsNode.Content[i+1], propRef.Value)
			}
		}
		p.propOrder[schema] = names
	}

	walkComposition := func(key string, refs openapi3.SchemaRefs) {
		seqNode := yamlMappingValue(node, key)
		if seqNode == nil || seqNode.Kind != yaml.SequenceNode {
			return
		}
		for i, item := range seqNode.Content {
			if i >= len(refs) || refs[i] == nil || refs[i].Value == nil {
				continue
			}
			p.walkSchemaOrder(item, refs[i].Value)
		}
	}
	walkComposition("allOf", schema.AllOf)
	walkComposition("oneOf", schema.OneOf)
	walkComposition("anyOf", schema.AnyOf)

	if schema.Items != nil && schema.Items.Value != nil {
		p.walkSchemaOrder(yamlMappingValue(node, "items"), schema.Items.Value)
	}
	if schema.AdditionalProperties.Schema != nil && schema.AdditionalProperties.Schema.Value != nil {
		p.walkSchemaOrder(yamlMappingValue(node, "additionalProperties"), schema.AdditionalProperties.Schema.Value)
	}
}

// yamlMappingValue returns the value node for key within YAML mapping node, or nil if
// node isn't a mapping or has no such key.
func yamlMappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// orderedPropertyNames returns schema's property names in the order they were declared
// in the source spec, recovered by buildPropertyOrder. Falls back to alphabetical order
// when the source order wasn't recovered for schema (e.g. a Swagger 2 source, or a
// schema reachable only through an external ref).
func (p *Parser) orderedPropertyNames(schema *openapi3.Schema) []string {
	order, ok := p.propOrder[schema]
	if !ok {
		names := make([]string, 0, len(schema.Properties))
		for name := range schema.Properties {
			names = append(names, name)
		}
		sort.Strings(names)
		return names
	}

	names := make([]string, 0, len(schema.Properties))
	seen := make(map[string]bool, len(order))
	for _, name := range order {
		if _, exists := schema.Properties[name]; exists {
			names = append(names, name)
			seen[name] = true
		}
	}
	// Cover any property present in the parsed schema but missing from the recovered
	// order (shouldn't normally happen); append alphabetically so nothing is dropped.
	if len(names) != len(schema.Properties) {
		var extra []string
		for name := range schema.Properties {
			if !seen[name] {
				extra = append(extra, name)
			}
		}
		sort.Strings(extra)
		names = append(names, extra...)
	}
	return names
}
