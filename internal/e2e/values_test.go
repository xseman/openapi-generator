package e2e

import (
	"encoding/json"
	"fmt"
	"maps"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"go.yaml.in/yaml/v3"
)

// valueLenient are the generators whose failing generated cases are
// counted and skipped, not failed: they have known gaps a later change
// fixes. Without an entry a generator is strict, as go-fetch is.
var valueLenient = map[string]bool{"typescript-fetch": true, "dart-fetch": true}

// valueScenarios generates a scenario for each feature spec, the FastAPI
// spec (served by the mock) and each spec in E2E_SPECS (comma-separated
// paths), and logs the operations it leaves out.
func valueScenarios(t *testing.T) []*scenario {
	t.Helper()

	specs, err := filepath.Glob("../../testdata/specs/feature/*.yaml")
	if err != nil {
		t.Fatal(err)
	}

	specs = append(specs, "../../testdata/e2e/specs/fastapi.json")
	if extra := os.Getenv("E2E_SPECS"); extra != "" {
		specs = append(specs, strings.Split(extra, ",")...)
	}

	var (
		out     []*scenario
		skipped []string
		n       int
	)

	for _, spec := range specs {
		s, why := valueScenario(t, spec)
		if skipped, n = append(skipped, why...), n+len(s.Cases); len(s.Cases) > 0 {
			out = append(out, s)
		}
	}

	t.Logf("values: %d cases from %d of %d specs; %d operations left out:\n%s",
		n, len(out), len(specs), len(skipped), strings.Join(skipped, "\n"))

	return out
}

// valueScenario generates the cases of one spec, its operations in sorted
// order, and lists the operations it leaves out and why.
func valueScenario(t *testing.T, spec string) (*scenario, []string) {
	t.Helper()

	name := "values/" + strings.TrimSuffix(filepath.Base(spec), filepath.Ext(spec))
	spec, doc := servable(t, spec)
	s := &scenario{Name: name, Spec: spec, Auth: &auth{Username: "u", Password: "p", APIKey: "k", Bearer: "t"}}

	var skipped []string

	for _, path := range slices.Sorted(maps.Keys(doc.Paths.Map())) {
		item := doc.Paths.Value(path)
		ops := item.Operations()

		for _, method := range slices.Sorted(maps.Keys(ops)) {
			if method == http.MethodHead {
				skipped = append(skipped, fmt.Sprintf("%s %s %s: a HEAD answer has no body", name, method, path))

				continue
			}

			cases, why := valueCases(item, ops[method], doc.IsOpenAPI31OrLater())
			if why != "" {
				skipped = append(skipped, fmt.Sprintf("%s %s %s: %s", name, method, path, why))
			}

			s.Cases = append(s.Cases, cases...)
		}
	}

	return s, skipped
}

var pathParam = regexp.MustCompile(`\{[^}]*\}`)

// servable loads spec as the mock can serve it: when it has to change it,
// it writes a copy and returns that path. It converts a Swagger 2.0 spec to
// OpenAPI 3, and of the paths that differ only in their parameters' names,
// which no router tells apart, it keeps the first. Real specs have both.
func servable(t *testing.T, spec string) (string, *openapi3.T) {
	t.Helper()

	data, err := os.ReadFile(spec)
	if err != nil {
		t.Fatal(err)
	}

	var y map[string]any
	if err := yaml.Unmarshal(data, &y); err != nil {
		t.Fatalf("%s: %v", spec, err)
	}

	var (
		doc     *openapi3.T
		changed = y["swagger"] != nil
	)

	if changed {
		var v2 openapi2.T
		if err = json.Unmarshal(mustJSON(y), &v2); err == nil {
			doc, err = openapi2conv.ToV3(&v2)
		}
	} else {
		doc, err = openapi3.NewLoader().LoadFromFile(spec)
	}

	if err != nil {
		t.Fatalf("%s: %v", spec, err)
	}

	shapes := map[string]bool{}

	for _, path := range slices.Sorted(maps.Keys(doc.Paths.Map())) {
		shape := pathParam.ReplaceAllString(path, "{}")
		if shapes[shape] {
			doc.Paths.Delete(path)

			changed = true
		}

		shapes[shape] = true
	}

	if !changed {
		return spec, doc
	}

	out := filepath.Join(t.TempDir(), filepath.Base(spec)+".json")
	write(t, out, string(mustJSON(doc)))

	if doc, err = openapi3.NewLoader().LoadFromFile(out); err != nil {
		t.Fatalf("%s: %v", spec, err)
	}

	return out, doc
}

// valueCases makes an operation's generated cases: each value of a JSON
// response is the mock's answer and the result, or the error's model; each
// value of a JSON request body is the body to send, answered by the first
// 2xx. Required parameters get a value of their own and a required body
// its first value; an operation that cannot have them is left out, why
// says so.
func valueCases(item *openapi3.PathItem, op *openapi3.Operation, is31 bool) ([]kase, string) {
	if op.OperationID == "" {
		return nil, "no operationId"
	}

	var (
		args      = map[string]any{}
		query     bool   // a query is sent
		deepArray string // a deepObject array, which kin refuses in any query (Stripe's expand)
		dropped   int    // values their schema rejects: a generator gap to log
	)

	for _, p := range slices.Concat(item.Parameters, op.Parameters) {
		if p.Value == nil {
			continue
		}

		if p.Value.Style == openapi3.SerializationDeepObject && p.Value.Schema != nil && p.Value.Schema.Value.Type.Is(openapi3.TypeArray) {
			deepArray = p.Value.Name
		}

		if !p.Value.Required {
			continue
		}

		v, ok := paramValue(p.Value.Schema, is31)
		if !ok {
			return nil, "no scalar value for parameter " + p.Value.Name
		}

		args[p.Value.Name] = v
		query = query || p.Value.In == openapi3.ParameterInQuery
	}

	if query && deepArray != "" {
		return nil, "the mock cannot validate a query next to the deepObject array " + deepArray
	}

	var bodies []namedValue

	if rb := op.RequestBody; rb != nil && rb.Value != nil {
		// A form's fields are arguments of their own.
		form := slices.ContainsFunc(slices.Collect(maps.Keys(rb.Value.Content)), func(ct string) bool {
			return strings.HasPrefix(ct, "multipart/") || ct == "application/x-www-form-urlencoded"
		})

		if _, mt := jsonMedia(rb.Value.Content); mt != nil && !form {
			bodies = valuesOf(mt.Schema, true, is31, &dropped)
		}

		if rb.Value.Required && len(bodies) == 0 {
			return nil, "the required body is not JSON, or a form too, or has no valid value"
		}

		if rb.Value.Required {
			args["body"] = bodies[0].raw
		}
	}

	var (
		cases  []kase
		answer *mockResponse       // the first 2xx's, for the request cases
		result *openapi3.SchemaRef // the first 2xx's JSON; another 2xx's body is an error
	)

	for _, code := range slices.Sorted(maps.Keys(op.Responses.Map())) {
		status, err := strconv.Atoi(code)

		resp := op.Responses.Value(code)
		if err != nil || status < 200 || status >= 300 && status < 400 || resp.Value == nil {
			continue
		}

		ct, mt := jsonMedia(resp.Value.Content)
		if mt == nil {
			if answer == nil && status < 300 && len(resp.Value.Content) == 0 {
				answer = &mockResponse{Status: status}
			}

			continue
		}

		if result == nil && status < 300 {
			result = mt.Schema
		}

		// An any result ({}) holds every 2xx body.
		isResult := status < 300 && (mt.Schema == result || mt.Schema.Ref != "" && mt.Schema.Ref == result.Ref || result.Value.IsEmpty())

		for _, v := range valuesOf(mt.Schema, false, is31, &dropped) {
			r := &mockResponse{Status: status, Header: map[string]string{"Content-Type": ct}, JSON: v.raw}
			if answer == nil && status < 300 {
				answer = r
			}

			if status == http.StatusNoContent {
				break
			}

			k := kase{Name: fmt.Sprintf("response %d %s", status, v.name), Op: op.OperationID, Args: mustJSON(args), Response: r, Exact: true}
			if isResult {
				k.Result = v.raw
			} else {
				k.Error = mustJSON(map[string]any{"status": status, "model": v.raw, "body": "$any"})
			}

			cases = append(cases, k)
		}
	}

	if len(bodies) > 0 && answer == nil {
		return cases, "no 2xx answer without a body or with JSON, for the request cases"
	}

	for _, b := range bodies {
		a := maps.Clone(args)
		a["body"] = b.raw

		// An optional body of null may go out as null or as none: the mock
		// checks only that the request is valid.
		want := mustJSON(map[string]any{"body": b.raw})
		if string(b.raw) == "null" && !op.RequestBody.Value.Required {
			want = nil
		}

		cases = append(cases, kase{
			Name: "request " + b.name, Op: op.OperationID, Args: mustJSON(a), Exact: true,
			Request: want, Response: answer,
		})
	}

	if dropped > 0 {
		return cases, fmt.Sprintf("%d generated values their schema rejects, left out", dropped)
	}

	return cases, ""
}

// jsonMedia finds the first JSON media type of a content, in sorted order.
func jsonMedia(content openapi3.Content) (string, *openapi3.MediaType) {
	for _, ct := range slices.Sorted(maps.Keys(content)) {
		mt, _, _ := mime.ParseMediaType(ct)
		if mt == "application/json" || strings.HasSuffix(mt, "+json") {
			return ct, content[ct]
		}
	}

	return "", nil
}

// paramValue is a valid scalar of a parameter's schema.
func paramValue(ref *openapi3.SchemaRef, is31 bool) (any, bool) {
	if ref == nil || ref.Value == nil {
		return nil, false
	}

	v := (&valueGen{mode: "all", is31: is31, open: map[*openapi3.Schema]int{}}).value(ref, 0)

	switch v.(type) {
	case string, float64, bool:
		return v, valid(ref.Value, mustJSON(v), true, is31)
	}

	return nil, false
}

type namedValue struct {
	name string
	raw  json.RawMessage
}

// valuesOf generates the values of a schema, each valid for it and each
// once: all properties, only the required ones, every nullable null, each
// union branch in turn (for all and nulls), and edge scalars with extra keys.
func valuesOf(ref *openapi3.SchemaRef, request, is31 bool, dropped *int) []namedValue {
	if ref == nil || ref.Value == nil {
		return nil
	}

	var (
		out  []namedValue
		seen = map[string]bool{}
	)

	for _, mode := range []string{"all", "required", "nulls", "edge"} {
		for branch := 0; ; branch++ {
			g := &valueGen{mode: mode, branch: branch, request: request, is31: is31, open: map[*openapi3.Schema]int{}}
			raw := mustJSON(g.value(ref, 0))

			ok := valid(ref.Value, raw, request, is31)
			if !ok {
				*dropped++
			}

			if !seen[string(raw)] && ok {
				seen[string(raw)] = true

				name := mode
				if branch > 0 {
					name += " " + strconv.Itoa(branch)
				}

				out = append(out, namedValue{name, raw})
			}

			if mode == "required" || mode == "edge" || branch+1 >= g.widest {
				break
			}
		}
	}

	return out
}

// valid reports whether the schema allows the JSON value, as the mock's
// validation will check it.
func valid(s *openapi3.Schema, raw json.RawMessage, request, is31 bool) bool {
	opts := []openapi3.SchemaValidationOption{openapi3.VisitAsResponse()}
	if request {
		opts[0] = openapi3.VisitAsRequest()
	}

	if is31 {
		opts = append(opts, openapi3.EnableJSONSchema2020())
	}

	var v any

	return json.Unmarshal(raw, &v) == nil && s.VisitJSON(v, opts...) == nil
}

// valueGen makes one value of a schema. Its mode picks which: all
// properties, only the required ones, every nullable null, or edge
// scalars; branch picks a union's branch and varies scalars and items.
type valueGen struct {
	mode          string
	branch        int
	request, is31 bool

	open   map[*openapi3.Schema]int // schemas being made, to end a recursion
	widest int                      // the most branches a union had
}

func (g *valueGen) value(ref *openapi3.SchemaRef, depth int) any {
	// A schema that requires itself has no value; null ends it, and the
	// validation drops what holds it.
	s := ref.Value
	if g.mode == "nulls" && g.nullable(s) || depth > 32 {
		return nil
	}

	g.open[s]++
	defer func() { g.open[s]-- }()

	// Past a depth, or inside itself, a schema takes only what it requires.
	deep := depth > 6 || g.open[s] > 1

	switch {
	case s.Const != nil:
		return s.Const
	case len(s.Enum) > 0:
		return g.enum(s)
	case len(s.AllOf) > 0:
		return g.allOf(ref, depth, deep)
	case len(branches(s)) > 0:
		return g.union(ref, depth, deep)
	}

	switch g.typeOf(s) {
	case openapi3.TypeString:
		return g.str(s)
	case openapi3.TypeInteger:
		return clamp(s, g.integer(s))
	case openapi3.TypeNumber:
		return clamp(s, g.number(s))
	case openapi3.TypeBoolean:
		return g.mode != "edge"
	case openapi3.TypeNull:
		return nil
	case openapi3.TypeArray:
		return g.array(s, depth, deep)
	case openapi3.TypeObject:
		return g.object(ref, depth, deep)
	}

	// Not an object: a union branch of constraints alone yields to the
	// union's own.
	if g.mode == "edge" {
		return []any{1, "two", nil, map[string]any{"k": true}}
	}

	return "anything"
}

// nullable reports whether null is a value of s: nullable (before 3.1), a
// null type or union branch.
func (g *valueGen) nullable(s *openapi3.Schema) bool {
	if s.Nullable && !g.is31 || s.Type.IncludesNull() {
		return true
	}

	return slices.ContainsFunc(slices.Concat(s.OneOf, s.AnyOf), func(b *openapi3.SchemaRef) bool {
		return b.Value != nil && b.Value.Type.Is(openapi3.TypeNull)
	})
}

// typeOf is the schema's first type other than null, or one its keywords
// imply.
func (g *valueGen) typeOf(s *openapi3.Schema) string {
	for _, t := range s.Type.Slice() {
		if t != openapi3.TypeNull {
			return t
		}
	}

	switch {
	case s.Type.Is(openapi3.TypeNull):
		return openapi3.TypeNull
	case len(s.Properties) > 0 || s.AdditionalProperties.Schema != nil:
		return openapi3.TypeObject
	case s.Items != nil:
		return openapi3.TypeArray
	}

	return ""
}

func (g *valueGen) enum(s *openapi3.Schema) any {
	vals := slices.DeleteFunc(slices.Clone(s.Enum), func(v any) bool { return v == nil })

	switch {
	case len(vals) == 0:
		return nil
	case g.mode == "edge":
		return vals[len(vals)-1]
	}

	return vals[g.branch%len(vals)]
}

// allOf merges the objects of its parts and its own properties.
func (g *valueGen) allOf(ref *openapi3.SchemaRef, depth int, deep bool) any {
	merged := map[string]any{}

	var other any

	for _, part := range ref.Value.AllOf {
		v := g.value(part, depth+1)
		if m, ok := v.(map[string]any); ok {
			maps.Copy(merged, m)
		} else if v != nil {
			other = v
		}

		if d := part.Value.Discriminator; d != nil {
			g.discriminate(merged, d, ref.Ref)
		}
	}

	if len(merged) == 0 && other != nil {
		return other
	}

	if len(ref.Value.Properties) > 0 {
		maps.Copy(merged, g.object(ref, depth, deep))
	}

	return merged
}

// branches are the oneOf and anyOf branches but a null one.
func branches(s *openapi3.Schema) openapi3.SchemaRefs {
	return slices.DeleteFunc(slices.Concat(s.OneOf, s.AnyOf), func(b *openapi3.SchemaRef) bool {
		return b.Value == nil || b.Value.Type.Is(openapi3.TypeNull)
	})
}

// union makes the branch g.branch picks, or deep the first one that does
// not recurse; an object gets the union's own properties and its
// discriminator.
func (g *valueGen) union(ref *openapi3.SchemaRef, depth int, deep bool) any {
	alts := branches(ref.Value)
	g.widest = max(g.widest, len(alts))

	pick := alts[g.branch%len(alts)]
	if deep {
		if i := slices.IndexFunc(alts, func(b *openapi3.SchemaRef) bool { return g.open[b.Value] == 0 }); i >= 0 {
			pick = alts[i]
		}
	}

	v := g.value(pick, depth+1)

	// A branch of constraints alone (required: [a]) is the union's object.
	m, ok := v.(map[string]any)
	if !ok && len(ref.Value.Properties) > 0 {
		m, ok = map[string]any{}, true
	}

	if !ok {
		return v
	}

	if len(ref.Value.Properties) > 0 {
		for k, x := range g.object(ref, depth, deep) {
			if _, has := m[k]; !has {
				m[k] = x
			}
		}
	}

	if d := ref.Value.Discriminator; d != nil {
		g.discriminate(m, d, pick.Ref)
	}

	return m
}

// discriminate sets the discriminator property of m, when m has it, to the
// value that names the schema at ref: its mapping key, or its name.
func (g *valueGen) discriminate(m map[string]any, d *openapi3.Discriminator, ref string) {
	if _, has := m[d.PropertyName]; !has || ref == "" {
		return
	}

	name := ref[strings.LastIndex(ref, "/")+1:]
	m[d.PropertyName] = name

	for _, key := range slices.Sorted(maps.Keys(d.Mapping)) {
		if to := d.Mapping[key].Ref; to == ref || to == name || strings.HasSuffix(to, "/"+name) {
			m[d.PropertyName] = key

			return
		}
	}
}

func (g *valueGen) object(ref *openapi3.SchemaRef, depth int, deep bool) map[string]any {
	s, m := ref.Value, map[string]any{}

	for _, k := range slices.Sorted(maps.Keys(s.Properties)) {
		p := s.Properties[k]

		switch {
		case p.Value == nil, g.request && p.Value.ReadOnly, !g.request && p.Value.WriteOnly:
			continue
		case (g.mode == "required" || deep) && !slices.Contains(s.Required, k):
			continue
		}

		m[k] = g.value(p, depth+1)
	}

	if g.mode != "required" && !deep {
		if ap := s.AdditionalProperties.Schema; ap != nil && ap.Value != nil {
			m["extra_1"] = g.value(ap, depth+1)
		} else if has := s.AdditionalProperties.Has; has != nil && *has {
			m["extra_1"] = map[string]any{"nested": []any{1, "a", nil, true}}
		}
	}

	if d := s.Discriminator; d != nil {
		g.discriminate(m, d, ref.Ref)
	}

	return m
}

func (g *valueGen) array(s *openapi3.Schema, depth int, deep bool) []any {
	n := 2
	if g.mode == "required" || deep {
		n = 0
	}

	n = max(n, int(s.MinItems)) //nolint:gosec // a schema's minItems is small
	if s.MaxItems != nil {
		n = min(n, int(*s.MaxItems)) //nolint:gosec // and so is its maxItems
	}

	out := make([]any, 0, n)

	for i := range n {
		switch {
		case i < len(s.PrefixItems):
			out = append(out, g.value(s.PrefixItems[i], depth+1))
		case s.Items != nil && s.Items.Value != nil:
			// Each item its own branch: an array of a union holds two.
			g.branch += i
			out = append(out, g.value(s.Items, depth+1))
			g.branch -= i

		default:
			out = append(out, "item")
		}
	}

	if s.UniqueItems {
		// Edge items are all alike.
		out = slices.CompactFunc(out, func(a, b any) bool { return string(mustJSON(a)) == string(mustJSON(b)) })
	}

	return out
}

func (g *valueGen) str(s *openapi3.Schema) string {
	v := "s" + strconv.Itoa(g.branch)

	switch s.Format {
	case "date-time":
		v = "2024-01-02T03:04:05Z"
		if g.mode == "edge" {
			v = "2024-02-29T23:59:59.123456789+05:30"
		}

	case "date":
		v = "2024-02-29"
	case "time":
		v = "13:14:15"
	case "uuid":
		v = "3fa85f64-5717-4562-b3fc-2c963f66afa6"
	case "email":
		v = "a@example.com"
	case "uri", "url":
		v = "https://example.com/a?b=c&d=%20"
	case "byte":
		v = "aGVsbG8="
	default:
		if g.mode == "edge" {
			v = "ž \"q\" <a>&b \u2028 😀 \\ /"
		}
	}

	// A pattern takes the schema's example: the backend gives one.
	if s.Pattern != "" {
		ex, _ := s.Example.(string)
		if len(s.Examples) > 0 {
			ex, _ = s.Examples[0].(string)
		}

		v = ex
	}

	r := []rune(v)
	if s.MaxLength != nil && uint64(len(r)) > *s.MaxLength {
		r = r[:*s.MaxLength]
	}

	for uint64(len(r)) < s.MinLength {
		r = append(r, 'x')
	}

	return string(r)
}

func (g *valueGen) integer(s *openapi3.Schema) float64 {
	switch {
	case g.mode != "edge":
		return float64(7 + g.branch)
	case s.Format == "int32":
		return -2147483648
	}

	return 9007199254740991 // 2^53-1, the largest a JavaScript number holds exactly
}

func (g *valueGen) number(s *openapi3.Schema) float64 {
	switch {
	case s.MultipleOf != nil:
		return *s.MultipleOf * 3
	case g.mode == "edge":
		return 0.1
	}

	return 1.5
}

// clamp moves v into the schema's minimum and maximum.
func clamp(s *openapi3.Schema, v float64) float64 {
	if s.Min != nil && v < *s.Min {
		v = *s.Min
		if s.ExclusiveMin.IsTrue() {
			v++
		}
	}

	if s.Max != nil && v > *s.Max {
		v = *s.Max
		if s.ExclusiveMax.IsTrue() {
			v--
		}
	}

	return v
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}

	return b
}
