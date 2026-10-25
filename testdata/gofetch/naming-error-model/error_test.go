package namingerrormodel

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"testing"
)

type status500 string

func (b status500) Do(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 500, Header: http.Header{}, Body: io.NopCloser(bytes.NewBufferString(string(b)))}, nil
}

// TestErrorModelKeys decodes an error body into the declared model only
// when it holds the model's required properties.
func TestErrorModelKeys(t *testing.T) {
	for body, want := range map[string]bool{`{"message":"boom"}`: true, `{"detail":"Not Found"}`: false, `not json`: false} {
		_, err := NewDefaultAPI(&Configuration{HTTPClient: status500(body)}).DoRisky(t.Context())

		var rerr *ResponseError
		if !errors.As(err, &rerr) {
			t.Fatalf("%s: error %v", body, err)
		}

		if m, ok := rerr.Model.(*Error); ok != want || (ok && m.Message != "boom") {
			t.Errorf("%s: model = %#v", body, rerr.Model)
		}
	}
}
