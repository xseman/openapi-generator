package golang

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xseman/openapi-generator/internal/codegen"
	"github.com/xseman/openapi-generator/internal/parser"
)

// parse runs a spec (a feature spec name or inline YAML) through the parser
// and the generator, as the pipeline does.
func parse(t *testing.T, spec string) (*FetchGenerator, *parser.Parser) {
	t.Helper()

	g := NewFetchGenerator()
	if err := g.ProcessOpts(); err != nil {
		t.Fatal(err)
	}

	p := parser.NewParser()
	p.GetTypeFunc, p.ToModelNameFunc, p.ToVarNameFunc = g.GetSchemaType, g.ToModelName, g.ToVarName
	p.SkipValidation = true

	var err error
	if strings.Contains(spec, "\n") {
		err = p.LoadFromData([]byte(spec))
	} else {
		err = p.LoadFromFile(filepath.Join("..", "..", "..", "testdata", "specs", "feature", spec+".yaml"))
	}

	if err != nil {
		t.Fatal(err)
	}

	return g, p
}

func models(t *testing.T, spec string) map[string]*codegen.CodegenModel {
	t.Helper()

	g, p := parse(t, spec)

	ms, err := p.GetModels()
	if err != nil {
		t.Fatal(err)
	}

	byName := make(map[string]*codegen.CodegenModel)
	for _, m := range g.PostProcessModels(ms) {
		byName[m.Classname] = m
	}

	return byName
}

// fields renders a struct model's fields as "Name Type Tag" lines.
func fields(m *codegen.CodegenModel) string {
	fs, _ := m.VendorExtensions["x-go-fields"].([]map[string]any)

	lines := make([]string, len(fs))
	for i, f := range fs {
		lines[i] = fmt.Sprintf("%v %v %v", f["name"], f["type"], f["tag"])
	}

	return strings.Join(lines, "\n")
}

func TestAllOfFlattened(t *testing.T) {
	tests := []struct {
		spec, model, want string
	}{
		{"composition-allof-inheritance", "Car", "ID string `json:\"id\"`\nMake string `json:\"make\"`\nDoors int `json:\"doors\"`\nTrunkVolumeLiters Opt[float64] `json:\"trunkVolumeLiters,omitzero\"`"},
		{"composition-allof-inheritance", "SportsCar", "ID string `json:\"id\"`\nMake string `json:\"make\"`\nDoors int `json:\"doors\"`\nTrunkVolumeLiters Opt[float64] `json:\"trunkVolumeLiters,omitzero\"`\nTopSpeedKph *int `json:\"topSpeedKph,omitempty\"`"},
		{"discriminator-mapping", "Cat", "PetType string `json:\"petType\"`\nName *string `json:\"name,omitempty\"`\nHuntingSkill *string `json:\"huntingSkill,omitempty\"`"},
		{"discriminator-mapping", "Dog", "PetType string `json:\"petType\"`\nName *string `json:\"name,omitempty\"`\nPackSize *int `json:\"packSize,omitempty\"`"},
		{"discriminator-self-ref", "LeafNode", "NodeType string `json:\"nodeType\"`\nChildren []TreeNode `json:\"children,omitzero\"`\nValue *string `json:\"value,omitempty\"`"},
	}

	for _, tt := range tests {
		if got := fields(models(t, tt.spec)[tt.model]); got != tt.want {
			t.Errorf("%s fields:\n%s\nwant:\n%s", tt.model, got, tt.want)
		}
	}
}

func TestEnums(t *testing.T) {
	ms := models(t, "enum-edge-values")

	tests := []struct {
		model, base string
		literals    []string
	}{
		{"Level", "int", []string{"-1", "1", "2"}},
		{"Severity", "string", []string{`"low"`, `"LOW"`, `"it's"`, `"a\\b"`}},
		{"State", "string", []string{`"placed"`, `"approved"`}},
		{"Rating", "int", []string{"1", "5"}},
	}

	for _, tt := range tests {
		enums, _ := ms[tt.model].VendorExtensions["x-go-enums"].([]map[string]any)
		if len(enums) != 1 || enums[0]["base"] != tt.base {
			t.Errorf("%s enums = %v, want one of base %s", tt.model, enums, tt.base)
			continue
		}

		values, _ := enums[0]["values"].([]map[string]any)

		var literals []string
		for _, v := range values {
			literals = append(literals, fmt.Sprint(v["literal"]))
		}

		if strings.Join(literals, " ") != strings.Join(tt.literals, " ") {
			t.Errorf("%s literals = %v, want %v", tt.model, literals, tt.literals)
		}
	}

	// A property that refers to an enum model takes its type, no copy.
	want := "Level Opt[Level] `json:\"level,omitzero\"`\nSeverity *Severity `json:\"severity,omitempty\"`\n" +
		"State *State `json:\"state,omitempty\"`\nRating *Rating `json:\"rating,omitempty\"`"
	if got := fields(ms["Levels"]); got != want {
		t.Errorf("Levels fields:\n%s\nwant:\n%s", got, want)
	}

	if enums, _ := ms["Levels"].VendorExtensions["x-go-enums"].([]map[string]any); len(enums) != 0 {
		t.Errorf("Levels declares enums %v", enums)
	}
}

func TestCyclesAndCollisions(t *testing.T) {
	ms := models(t, `openapi: 3.0.0
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    A:
      type: object
      required: [b]
      properties: {b: {$ref: "#/components/schemas/B"}}
    B:
      type: object
      required: [a]
      properties: {a: {$ref: "#/components/schemas/A"}}
    Node:
      type: object
      required: [next, id, ID]
      properties:
        next: {$ref: "#/components/schemas/Node"}
        id: {type: string}
        ID: {type: string}
        parent: {allOf: [{$ref: "#/components/schemas/Node"}], nullable: true}
    Names:
      type: object
      required: [foo2, foo, Foo]
      properties: {foo2: {type: string}, foo: {type: string}, Foo: {type: string}}
    Owner:
      type: object
      required: [pet]
      properties:
        pet: {$ref: "#/components/schemas/Pet"}
        pets: {type: array, items: {$ref: "#/components/schemas/Pet"}}
    Pet:
      oneOf: [{$ref: "#/components/schemas/A"}, {$ref: "#/components/schemas/B"}]
`)

	for model, want := range map[string]string{
		"A":     "B B `json:\"b\"`",
		"B":     "A *A `json:\"a\"`",
		"Node":  "Next *Node `json:\"next\"`\nID string `json:\"id\"`\nID2 string `json:\"ID\"`\nParent Opt[*Node] `json:\"parent,omitzero\"`",
		"Names": "Foo2 string `json:\"foo2\"`\nFoo string `json:\"foo\"`\nFoo3 string `json:\"Foo\"`",
		"Owner": "Pet Pet `json:\"pet\"`\nPets []Pet `json:\"pets,omitzero\"`",
	} {
		if got := fields(ms[model]); got != want {
			t.Errorf("%s fields:\n%s\nwant:\n%s", model, got, want)
		}
	}
}

func TestResponseTypes(t *testing.T) {
	g, p := parse(t, `openapi: 3.0.0
info: {title: t, version: "1"}
paths:
  /pet:
    get:
      operationId: getPet
      responses:
        "200": {description: ok, content: {application/json: {schema: {$ref: "#/components/schemas/Pet"}}}}
        "4XX": {description: client, content: {application/json: {schema: {type: array, items: {type: string}}}}}
  /pets:
    get:
      operationId: listPets
      responses:
        "200": {description: ok, content: {application/json: {schema: {type: array, items: {$ref: "#/components/schemas/Pet"}}}}}
  /photo:
    get:
      operationId: getPhoto
      responses:
        "200": {description: ok, content: {image/png: {schema: {type: string, format: binary}}}}
  /ping:
    get:
      operationId: ping
      responses:
        "204": {description: ok}
  /job:
    post:
      operationId: startJob
      responses:
        "201": {description: ok, content: {application/json: {schema: {$ref: "#/components/schemas/Pet"}}}}
        "202": {description: accepted}
    get:
      operationId: listJobs
      responses:
        "200": {description: ok, content: {application/json: {schema: {type: array, items: {type: string}}}}}
        "204": {description: none}
components:
  schemas:
    Pet: {type: object, properties: {name: {type: string}}}
    GetPetParams: {type: object, properties: {id: {type: string}}}
`)

	byTag, err := p.GetOperations()
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"getPet": "Pet", "listPets": "[]Pet", "getPhoto": "[]byte", "ping": "",
		"startJob": "*Pet", "listJobs": "[]string",
	}

	// The GetPetParams schema takes the name of getPet's Params type.
	ms, err := p.GetModels()
	if err != nil {
		t.Fatal(err)
	}

	g.PostProcessModels(ms)

	for _, op := range g.PostProcessOperations(byTag["default"]) {
		if got := op.VendorExtensions["x-go-type"]; got != want[op.OperationId] {
			t.Errorf("%s returns %q, want %q", op.OperationId, got, want[op.OperationId])
		}

		if params := op.VendorExtensions["x-go-params"]; op.OperationId == "getPet" && params != "GetPetParams2" {
			t.Errorf("getPet params = %v, want GetPetParams2 next to the GetPetParams model", params)
		}

		if op.OperationId == "getPet" {
			errs, _ := op.VendorExtensions["x-go-errors"].(map[string]any)
			if ranges, _ := errs["ranges"].([]map[string]any); len(ranges) != 1 || ranges[0]["digit"] != "4" || ranges[0]["type"] != "[]string" {
				t.Errorf("getPet error ranges = %v, want 4XX of []string", errs["ranges"])
			}
		}
	}
}

// TestAnyOfImplicitDiscriminator checks that an anyOf with a discriminator
// and no mapping maps each member's schema name to it, as a oneOf does.
func TestAnyOfImplicitDiscriminator(t *testing.T) {
	ms := models(t, `openapi: 3.0.0
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    Pet:
      anyOf: [{$ref: "#/components/schemas/Cat"}, {$ref: "#/components/schemas/Dog"}]
      discriminator: {propertyName: kind}
    Cat: {type: object, required: [kind], properties: {kind: {type: string}}}
    Dog: {type: object, required: [kind], properties: {kind: {type: string}}}
`)

	d, _ := ms["Pet"].VendorExtensions["x-go-discriminator"].(map[string]any)
	if got := fmt.Sprint(d["mappings"]); got != `[map[type:Cat value:"Cat"] map[type:Dog value:"Dog"]]` {
		t.Errorf("mappings = %s", got)
	}
}

// TestEnumRefParam types a parameter that refers to an enum model as that
// model, not as its base type.
func TestEnumRefParam(t *testing.T) {
	g, p := parse(t, `openapi: 3.0.0
info: {title: t, version: "1"}
paths:
  /paint/{color}:
    get:
      operationId: paint
      parameters:
        - {name: color, in: path, required: true, schema: {$ref: "#/components/schemas/Color"}}
        - {name: also, in: query, schema: {type: array, items: {$ref: "#/components/schemas/Color"}}}
        - {name: either, in: query, schema: {$ref: "#/components/schemas/Either"}}
      responses: {"204": {description: ok}}
components:
  schemas:
    Color: {type: string, enum: [red, light green]}
    Either: {oneOf: [{type: string}, {type: integer}]}
`)

	ms, err := p.GetModels()
	if err != nil {
		t.Fatal(err)
	}

	g.PostProcessModels(ms)

	byTag, err := p.GetOperations()
	if err != nil {
		t.Fatal(err)
	}

	op := g.PostProcessOperations(byTag["default"])[0]
	fs, _ := op.VendorExtensions["x-go-fields"].([]map[string]any)

	// A union stays raw JSON, which the runtime sends as the scalar it holds.
	if got := fmt.Sprint(fs[0]["type"], " ", fs[1]["type"], " ", fs[2]["type"]); got != "Color []Color json.RawMessage" {
		t.Errorf("param types = %s, want Color []Color json.RawMessage", got)
	}
}
