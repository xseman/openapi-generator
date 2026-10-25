package e2e

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"maps"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
)

// mock answers each case with its scenario response, after validating the
// request against the spec and recording it as the wire carried it.
type mock struct {
	srv    *httptest.Server
	router routers.Router
	cases  []kase

	mu       sync.Mutex
	current  int
	received map[int]map[string]any
}

// A JSON-lines body is text to the validator, as to a client.
func init() {
	openapi3filter.RegisterBodyDecoder("application/x-ndjson", openapi3filter.PlainBodyDecoder)
}

// newMock serves the spec at a fresh local URL.
func newMock(t *testing.T, specPath string, cases []kase) *mock {
	t.Helper()

	loader := openapi3.NewLoader()

	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		t.Fatal(err)
	}

	m := &mock{cases: cases, current: -1, received: map[int]map[string]any{}}
	m.srv = httptest.NewServer(m)
	t.Cleanup(m.srv.Close)

	// A server-side request carries no host: match the path alone.
	doc.Servers = openapi3.Servers{{URL: "/"}}

	if m.router, err = legacy.NewRouter(doc); err != nil {
		t.Fatal(err)
	}

	return m
}

func (m *mock) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if n, ok := strings.CutPrefix(r.URL.Path, "/__e2e/case/"); ok {
		m.mu.Lock()
		m.current, _ = strconv.Atoi(n)
		m.mu.Unlock()

		return
	}

	body, _ := io.ReadAll(r.Body)
	got := record(r, body)

	input, err := m.validate(r.Context(), r, body)
	if err != nil {
		got["invalid"] = err.Error()
	}

	m.mu.Lock()
	i := m.current

	if _, seen := m.received[i]; !seen {
		m.received[i] = got
	}
	m.mu.Unlock()

	if i < 0 || i >= len(m.cases) || m.cases[i].Response == nil {
		http.Error(w, "no response for this case", http.StatusNotImplemented)

		return
	}

	status, header, out := m.cases[i].Response.encode()

	// The scenario's response must be one the spec allows, or a case could
	// pass on a body no server would send.
	if input != nil && !m.cases[i].Response.OffSpec {
		err := openapi3filter.ValidateResponse(r.Context(), &openapi3filter.ResponseValidationInput{
			RequestValidationInput: input, Status: status, Header: header, Body: io.NopCloser(bytes.NewReader(out)),
			Options: &openapi3filter.Options{IncludeResponseStatus: true, MultiError: true},
		})
		if err != nil {
			m.mu.Lock()
			got["responseInvalid"] = err.Error()
			m.mu.Unlock()
		}
	}

	maps.Copy(w.Header(), header)
	w.WriteHeader(status)
	_, _ = w.Write(out)
}

// validate checks the request against its operation in the spec: path,
// parameters by style and explode, and the body by its media type. It
// returns the input the response is validated with, nil without a route.
func (m *mock) validate(ctx context.Context, r *http.Request, body []byte) (*openapi3filter.RequestValidationInput, error) {
	r = r.Clone(ctx)
	r.Body = io.NopCloser(bytes.NewReader(body))

	route, pathParams, err := m.router.FindRoute(r)
	if err != nil {
		return nil, err
	}

	input := &openapi3filter.RequestValidationInput{
		Request:    r,
		PathParams: pathParams,
		Route:      route,
		Options:    &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc, MultiError: true},
	}

	return input, openapi3filter.ValidateRequest(ctx, input)
}

// encode is the response as sent: the status (200 by default), headers and
// the body, its Content-Type set from its kind unless the case gives one.
func (resp *mockResponse) encode() (int, http.Header, []byte) {
	status := resp.Status
	if status == 0 {
		status = http.StatusOK
	}

	header := http.Header{}
	for k, v := range resp.Header {
		header.Set(k, v)
	}

	var (
		body        []byte
		contentType string
	)

	switch {
	case len(resp.JSON) > 0:
		body, contentType = resp.JSON, "application/json"
	case resp.Text != nil:
		body, contentType = []byte(*resp.Text), "text/plain"
	case resp.Base64 != "":
		body, _ = base64.StdEncoding.DecodeString(resp.Base64)
		contentType = "application/octet-stream"
	}

	if contentType != "" && header.Get("Content-Type") == "" {
		header.Set("Content-Type", contentType)
	}

	return status, header, body
}

// ignoredHeaders are set by the HTTP client itself, not by the generated
// code a scenario checks.
var ignoredHeaders = map[string]bool{"Accept-Encoding": true, "Content-Length": true, "User-Agent": true, "Cookie": true}

// record is the request as a scenario's "request" reads it: the method,
// the decoded path, query values, headers, cookies and the body by its
// media type (JSON as JSON, a form as lists of values, multipart as lists
// of parts, text as a string, anything else as base64).
func record(r *http.Request, body []byte) map[string]any {
	query := map[string]any{}
	for k, vs := range r.URL.Query() {
		query[k] = strs(vs)
	}

	header := map[string]any{}

	for k, vs := range r.Header {
		if !ignoredHeaders[k] {
			header[k] = strs(vs)
		}
	}

	cookie := map[string]any{}
	for _, c := range r.Cookies() {
		cookie[c.Name] = c.Value
	}

	got := map[string]any{
		"method":  r.Method,
		"path":    r.URL.Path,
		"rawPath": r.URL.EscapedPath(),
		"query":   query,
		"header":  header,
		"cookie":  cookie,
	}

	if len(body) > 0 {
		got["body"] = recordBody(r.Header.Get("Content-Type"), body)
	}

	return got
}

func recordBody(contentType string, body []byte) any {
	mt, params, _ := mime.ParseMediaType(contentType)

	switch {
	case mt == "application/json" || strings.HasSuffix(mt, "+json"):
		return decodeJSON(body)
	case mt == "application/x-www-form-urlencoded":
		values, _ := url.ParseQuery(string(body))

		form := map[string]any{}
		for k, vs := range values {
			form[k] = strs(vs)
		}

		return form

	case mt == "multipart/form-data":
		return recordParts(body, params["boundary"])
	case strings.HasPrefix(mt, "text/"):
		return string(body)
	}

	return map[string]any{"$base64": base64.StdEncoding.EncodeToString(body)}
}

// recordParts lists each multipart field's parts: a plain part as its
// text, a file as {filename, contentType, text or base64}.
func recordParts(body []byte, boundary string) any {
	parts := map[string]any{}
	mr := multipart.NewReader(bytes.NewReader(body), boundary)

	for {
		p, err := mr.NextPart()
		if err != nil {
			return parts
		}

		data, _ := io.ReadAll(p)

		var v any = string(data)

		if p.FileName() != "" {
			f := map[string]any{"filename": p.FileName(), "contentType": p.Header.Get("Content-Type")}
			if utf8.Valid(data) {
				f["text"] = string(data)
			} else {
				f["base64"] = base64.StdEncoding.EncodeToString(data)
			}

			v = f
		} else if mt, _, _ := mime.ParseMediaType(p.Header.Get("Content-Type")); mt == "application/json" || strings.HasSuffix(mt, "+json") {
			v = map[string]any{"json": decodeJSON(data)}
		}

		list, _ := parts[p.FormName()].([]any)
		parts[p.FormName()] = append(list, v)
	}
}

func strs(vs []string) []any {
	out := make([]any, len(vs))
	for i, v := range vs {
		out[i] = v
	}

	return out
}
