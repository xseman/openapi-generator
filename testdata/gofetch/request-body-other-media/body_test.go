package requestbodyothermedia

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// echo answers with the request's body and records the request.
type echo struct{ req *http.Request }

func (e *echo) Do(req *http.Request) (*http.Response, error) {
	e.req = req

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	h := http.Header{"Content-Type": {req.Header.Get("Content-Type")}}

	return &http.Response{StatusCode: 200, Header: h, Body: io.NopCloser(bytes.NewReader(body))}, nil
}

func TestTextAndRawBodies(t *testing.T) {
	e := &echo{}
	api := NewDefaultAPI(&Configuration{HTTPClient: e})

	note, err := api.PostNote(t.Context(), PostNoteParams{Body: "line 1\nčo"})
	if err != nil || note != "line 1\nčo" || e.req.Header.Get("Content-Type") != "text/plain" {
		t.Errorf("text: %q, %v, %s", note, err, e.req.Header.Get("Content-Type"))
	}

	blob, err := api.PutBlob(t.Context(), PutBlobParams{Body: bytes.NewReader([]byte{0, 0xff})})
	if err != nil || !bytes.Equal(blob, []byte{0, 0xff}) || e.req.Header.Get("Content-Type") != "application/octet-stream" {
		t.Errorf("raw: %v, %v", blob, err)
	}
}

func TestMultipartObjectPart(t *testing.T) {
	e := &echo{}
	api := NewDefaultAPI(&Configuration{HTTPClient: e})

	err := api.PostReport(t.Context(), PostReportParams{
		Title:       "q3",
		Meta:        &Meta{Author: Ptr("ann"), Pages: Ptr(3)},
		Attachments: []File{{Name: "a.txt", Body: strings.NewReader("a")}, {Body: strings.NewReader("b")}},
	})
	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest(http.MethodPost, "/", bytes.NewReader(e.body(t)))
	req.Header = e.req.Header

	if err := req.ParseMultipartForm(1 << 20); err != nil {
		t.Fatal(err)
	}

	// A File without a Name is named after its field.
	f := req.MultipartForm
	if f.Value["title"][0] != "q3" || f.Value["meta"][0] != `{"author":"ann","pages":3}` ||
		len(f.File["attachments"]) != 2 || f.File["attachments"][1].Filename != "attachments" {
		t.Errorf("form = %v, files %v", f.Value, f.File["attachments"])
	}
}

// body is the body the last request sent, read again.
func (e *echo) body(t *testing.T) []byte {
	t.Helper()

	b, err := e.req.GetBody()
	if err != nil {
		t.Fatal(err)
	}

	data, _ := io.ReadAll(b)

	return data
}

// TestConfiguration checks BasePath, the default headers, the bearer token
// and that an editor's error stops the request.
func TestConfiguration(t *testing.T) {
	var got *http.Request

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	cfg := &Configuration{
		BasePath: srv.URL + "/v2",
		Headers:  http.Header{"X-Trace": {"t1"}},
		AccessToken: func(_ context.Context, op, name string, _ []string) (string, error) {
			return op + ":" + name, nil
		},
	}

	if _, err := NewDefaultAPI(cfg).PostNote(t.Context(), PostNoteParams{Body: "x"}); err != nil {
		t.Fatal(err)
	}

	if got.URL.Path != "/v2/notes" || got.Header.Get("X-Trace") != "t1" || got.Header.Get("Authorization") != "Bearer postNote:bearer" {
		t.Errorf("request %s %v", got.URL.Path, got.Header)
	}

	stop := errors.New("stop")
	_, err := NewDefaultAPI(cfg).PostNote(t.Context(), PostNoteParams{Body: "x"}, func(context.Context, *http.Request) error { return stop })

	if !errors.Is(err, stop) {
		t.Errorf("editor error = %v", err)
	}

	// The spec's server, when BasePath is not set.
	req, err := NewDefaultAPI(nil).PutBlobRequest(t.Context(), PutBlobParams{Body: strings.NewReader("")})
	if err != nil || req.URL.String() != "http://api.example.com/v1/blobs" {
		t.Errorf("default base: %v, %v", req.URL, err)
	}
}
