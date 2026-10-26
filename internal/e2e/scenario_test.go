// Package e2e runs generated clients against servers: a mock that validates
// every request against the spec and records it, and real backends. The
// cases are the language-neutral scenarios in testdata/e2e/scenarios; each
// generator has a runner that turns them into calls of its client. Run with
// `make e2e`.
package e2e

import (
	"bytes"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"go.yaml.in/yaml/v3"
)

// scenario is one testdata/e2e/scenarios file: the cases for one spec, sent
// to the mock (spec) or to a running backend (backend).
type scenario struct {
	Name    string `json:"-"`
	Spec    string `json:"spec"`
	Backend string `json:"backend"`
	Auth    *auth  `json:"auth"`
	Cases   []kase `json:"cases"`

	// Skip skips every case for a generator, one whose client for the spec
	// does not even compile; a case's own skip says more.
	Skip map[string]string `json:"skip"`
}

type auth struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	APIKey   string `json:"apiKey,omitempty"`
	Bearer   string `json:"bearer,omitempty"`
}

// kase is one call: the operation and its arguments by the spec's names,
// what the mock must receive and answer, and what the client must return.
type kase struct {
	Name     string            `json:"name"`
	Op       string            `json:"op"`
	Args     json.RawMessage   `json:"args"`
	Auth     *auth             `json:"auth"`
	Save     map[string]string `json:"save"`
	Request  json.RawMessage   `json:"request"`
	Response *mockResponse     `json:"response"`
	Result   json.RawMessage   `json:"result"` // null is a null result, absent is unchecked
	Error    json.RawMessage   `json:"error"`

	// Exact compares the result, the error and the request's body with no
	// other keys allowed: a generated case (values_test.go) sets it.
	Exact bool              `json:"-"`
	Skip  map[string]string `json:"skip"`
}

// mockResponse is what the mock answers a case with: a status, headers and
// one of a JSON, text or base64 body. OffSpec marks one the spec does not
// allow on purpose (an undeclared status), which is otherwise an error.
type mockResponse struct {
	OffSpec bool              `json:"offSpec"`
	Status  int               `json:"status"`
	Header  map[string]string `json:"header"`
	JSON    json.RawMessage   `json:"json"`
	Text    *string           `json:"text"`
	Base64  string            `json:"base64"`
}

const scenarioDir = "../../testdata/e2e/scenarios"

// loadScenarios reads every scenario file, YAML turned into JSON so the
// values compare as the wire's (numbers as json.Number).
func loadScenarios(t *testing.T) []*scenario {
	t.Helper()

	files, err := filepath.Glob(filepath.Join(scenarioDir, "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no scenarios in %s: %v", scenarioDir, err)
	}

	var out []*scenario

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}

		var y any
		if err := yaml.Unmarshal(data, &y); err != nil {
			t.Fatalf("%s: %v", file, err)
		}

		j, err := json.Marshal(y)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}

		s := &scenario{Name: strings.TrimSuffix(filepath.Base(file), ".yaml")}

		dec := json.NewDecoder(bytes.NewReader(j))
		dec.UseNumber()
		dec.DisallowUnknownFields()

		if err := dec.Decode(s); err != nil {
			t.Fatalf("%s: %v", file, err)
		}

		if s.Spec != "" {
			s.Spec = filepath.Join(scenarioDir, s.Spec)
		}

		out = append(out, s)
	}

	return out
}

// opParams lists, per operationId, the names a case's args may use, in the
// order of the spec: the parameters and, for a form body, its properties
// (sorted). A runner uses them to tell the body's field from the
// parameters', and their order to tell apart two names that a generator
// gives one identifier, the second one numbered (itemId, itemId2). kin
// does not keep a schema's property order, so two form fields of one
// identifier must stand in the spec in sorted order too.
func opParams(doc *openapi3.T) map[string][]string {
	out := map[string][]string{}

	for _, item := range doc.Paths.Map() {
		for _, op := range item.Operations() {
			var names []string

			for _, p := range slices.Concat(item.Parameters, op.Parameters) {
				if p.Value != nil {
					names = append(names, p.Value.Name)
				}
			}

			if op.RequestBody != nil && op.RequestBody.Value != nil {
				for ct, mt := range op.RequestBody.Value.Content {
					form := strings.HasPrefix(ct, "multipart/") || ct == "application/x-www-form-urlencoded"
					if form && mt.Schema != nil && mt.Schema.Value != nil {
						names = append(names, slices.Sorted(maps.Keys(mt.Schema.Value.Properties))...)
					}
				}
			}

			out[op.OperationID] = names
		}
	}

	return out
}

// opBodies lists the responses of each operation that have a body, by
// their key in the spec: "200", "404", "4XX", "default".
func opBodies(doc *openapi3.T) map[string][]string {
	out := map[string][]string{}

	for _, item := range doc.Paths.Map() {
		for _, op := range item.Operations() {
			if op.Responses == nil {
				continue
			}

			for code, resp := range op.Responses.Map() {
				if resp.Value != nil && len(resp.Value.Content) > 0 {
					out[op.OperationID] = append(out[op.OperationID], code)
				}
			}

			slices.Sort(out[op.OperationID])
		}
	}

	return out
}

// decodeJSON decodes JSON keeping numbers exact.
func decodeJSON(b []byte) any {
	var v any

	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()

	if err := dec.Decode(&v); err != nil {
		return string(b)
	}

	return v
}
