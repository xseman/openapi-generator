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
  /records:
    post:
      operationId: setRecords
      responses:
        "200": {description: ok, content: {application/json: {schema: {type: object, properties: {total: {type: integer}}}}}}
        "201": {description: new, content: {application/json: {schema: {type: object, properties: {total: {type: integer}}}}}}
        "206": {description: some, content: {application/json: {schema: {type: array, items: {type: object, properties: {id: {type: string}}}}}}}
        "207": {description: partly, content: {application/json: {schema: {type: object, properties: {errors: {type: array, items: {type: string}}}}}}}
  /maybe:
    get:
      operationId: getMaybe
      responses:
        "200": {description: ok, content: {application/json: {schema: {$ref: "#/components/schemas/Maybe"}}}}
        "204": {description: none}
    put:
      operationId: listMaybe
      responses:
        "200": {description: ok, content: {application/json: {schema: {type: array, items: {$ref: "#/components/schemas/Maybe"}}}}}
components:
  schemas:
    Pet: {type: object, properties: {name: {type: string}}}
    GetPetParams: {type: object, properties: {id: {type: string}}}
    Maybe: {type: object, nullable: true, properties: {name: {type: string}}}
`)

	byTag, err := p.GetOperations()
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"getPet": "Pet", "listPets": "[]Pet", "getPhoto": "[]byte", "ping": "",
		"startJob": "*Pet", "listJobs": "[]string", "setRecords": "SetRecordsResponse",
		"getMaybe": "*Maybe", "listMaybe": "[]*Maybe",
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

		// Another inline object of a 2xx is a type of its own, not the result;
		// one of the result's shape is the result.
		if other, _ := op.VendorExtensions["x-go-other-2xx"].(map[string]any); op.OperationId == "setRecords" {
			codes, _ := other["codes"].([]map[string]any)
			if got := fmt.Sprint(codes); len(codes) != 2 || codes[0]["type"] != "[]SetRecords206Response" || codes[1]["type"] != "SetRecords207Response" {
				t.Errorf("setRecords other 2xx = %s, want 206 of []SetRecords206Response and 207 of SetRecords207Response", got)
			}
		}

		if doc := fmt.Sprint(op.VendorExtensions["x-go-doc"]); op.OperationId == "getMaybe" && !strings.Contains(doc, "nil for a 204") {
			t.Errorf("getMaybe doc = %s, want the 204 that leaves it nil", doc)
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

// TestModelShapes covers the model kinds the feature specs leave out:
// aliases of each type, nullable items, bool and float enums, union
// variants of arrays and maps, implied discriminators that do not hold, and
// a property name that needs a quoted tag.
func TestModelShapes(t *testing.T) {
	ms := models(t, `openapi: 3.0.0
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    Pet: {type: object, required: [kind], properties: {kind: {type: string, enum: [pet]}}}
    Pets: {type: array, items: {$ref: "#/components/schemas/Pet"}}
    Counts: {type: object, additionalProperties: {type: integer}}
    Name: {type: string}
    Age: {type: integer}
    Weight: {type: number}
    Flag: {type: boolean}
    Free: {type: object}
    Sparse:
      type: object
      properties:
        list: {type: array, items: {type: integer, nullable: true}}
        map: {type: object, additionalProperties: {type: string, nullable: true}}
        "a`+"`"+`b": {type: string}
    Toggle: {type: boolean, enum: [true, false]}
    Ratio: {type: number, enum: [0.5, 1, 1.5]}
    Mixed:
      oneOf:
        - $ref: "#/components/schemas/Pet"
        - {type: array, items: {$ref: "#/components/schemas/Pet"}}
        - {type: object, additionalProperties: {type: string}}
    Same:
      oneOf:
        - $ref: "#/components/schemas/Pet"
        - $ref: "#/components/schemas/Other"
    Other: {type: object, required: [kind], properties: {kind: {type: string, enum: [pet]}}}
`)

	for model, want := range map[string]string{
		"Pets": "[]Pet", "Counts": "map[string]int", "Name": "string", "Age": "int",
		"Weight": "float64", "Flag": "bool", "Free": "map[string]any",
	} {
		if got := ms[model].VendorExtensions["x-go-alias-type"]; got != want {
			t.Errorf("%s = %v, want %s", model, got, want)
		}
	}

	if got, want := fields(ms["Sparse"]), "List []*int `json:\"list,omitzero\"`\nMap map[string]*string `json:\"map,omitzero\"`\n"+
		"AB *string \"json:\\\"a`b,omitempty\\\"\""; got != want {
		t.Errorf("Sparse fields:\n%s\nwant:\n%s", got, want)
	}

	for model, want := range map[string]string{"Toggle": "bool", "Ratio": "float64"} {
		enums, _ := ms[model].VendorExtensions["x-go-enums"].([]map[string]any)
		if len(enums) != 1 || enums[0]["base"] != want {
			t.Errorf("%s enum = %v, want base %s", model, enums, want)
		}
	}

	variants, _ := ms["Mixed"].VendorExtensions["x-go-variants"].([]map[string]any)

	var methods []string
	for _, v := range variants {
		methods = append(methods, fmt.Sprint(v["method"]))
	}

	if got := strings.Join(methods, " "); got != "Pet PetList StringMap" {
		t.Errorf("Mixed variants = %s", got)
	}

	// Pet and Other both pin kind to "pet": kind tells them apart from no other.
	if d := ms["Same"].VendorExtensions["x-go-discriminator"]; d != false {
		t.Errorf("Same discriminator = %v, want none", d)
	}
}

// TestOperationShapes covers operation naming the feature specs leave out:
// a FooRaw next to Foo, an operation under two tags naming its inline types
// once, and an array of inline objects as a parameter.
func TestOperationShapes(t *testing.T) {
	g, p := parse(t, `openapi: 3.0.0
info: {title: t, version: "1"}
paths:
  /foo:
    get:
      operationId: foo
      responses: {"204": {description: ok}}
  /foo/raw:
    get:
      operationId: fooRaw
      responses: {"204": {description: ok}}
  /shared:
    post:
      operationId: shared
      tags: [a, b]
      parameters:
        - name: points
          in: query
          content:
            application/json:
              schema: {type: array, items: {type: object, properties: {x: {type: integer}}}}
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: {type: object, properties: {id: {type: string}}}
`)

	byTag, err := p.GetOperations()
	if err != nil {
		t.Fatal(err)
	}

	names := map[string]string{}
	for _, op := range g.PostProcessOperations(byTag["default"]) {
		names[op.OperationId] = fmt.Sprint(op.VendorExtensions["x-go-name"])
	}

	if names["foo"] != "Foo" || names["fooRaw"] != "FooRawMethod" {
		t.Errorf("names = %v, want Foo and FooRawMethod", names)
	}

	for _, tag := range []string{"a", "b"} {
		op := g.PostProcessOperations(byTag[tag])[0]

		if got := op.VendorExtensions["x-go-type"]; got != "SharedResponse" {
			t.Errorf("tag %s: result %v, want SharedResponse each time", tag, got)
		}

		fs, _ := op.VendorExtensions["x-go-fields"].([]map[string]any)
		if got := fs[0]["type"]; got != "[]SharedPoints" {
			t.Errorf("tag %s: points %v, want []SharedPoints", tag, got)
		}
	}
}

// TestBodyNamedBlob: a model named Blob, the parser's name for binary, is
// still a JSON body; only a binary body is an io.Reader.
func TestBodyNamedBlob(t *testing.T) {
	g, p := parse(t, `openapi: 3.0.0
info: {title: t, version: "1"}
paths:
  /blob:
    post:
      operationId: putBlob
      requestBody: {required: true, content: {application/json: {schema: {$ref: "#/components/schemas/Blob"}}}}
      responses: {"204": {description: ok}}
  /raw:
    post:
      operationId: putRaw
      requestBody: {required: true, content: {application/octet-stream: {schema: {type: string, format: binary}}}}
      responses: {"204": {description: ok}}
components:
  schemas:
    Blob: {type: object, properties: {data: {type: string}}}
`)

	byTag, err := p.GetOperations()
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{"putBlob": "Blob", "putRaw": "io.Reader"}

	for _, op := range g.PostProcessOperations(byTag["default"]) {
		fs, _ := op.VendorExtensions["x-go-fields"].([]map[string]any)
		if got := fs[0]["type"]; got != want[op.OperationId] {
			t.Errorf("%s body = %v, want %s", op.OperationId, got, want[op.OperationId])
		}
	}
}
