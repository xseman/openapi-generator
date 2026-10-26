package responsesmixed2xx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMixed2xx(t *testing.T) {
	status, ctype, body := 0, "", ""

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", ctype)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	api := NewDefaultAPI(&Configuration{BasePath: srv.URL})

	status, ctype, body = 201, "application/json", `{"id":7}`
	if o, err := api.PlaceOrder(t.Context()); err != nil || o.ID != 7 {
		t.Errorf("201: %+v, %v", o, err)
	}

	// The 202 body is a Problem, which an Order cannot hold.
	status, body = 202, `{"title":"queued","status":202}`
	_, err := api.PlaceOrder(t.Context())

	var rerr *ResponseError
	if !errors.As(err, &rerr) || rerr.StatusCode != 202 {
		t.Fatalf("202: %v", err)
	}

	if p, ok := rerr.Model.(*Problem); !ok || p.Title != "queued" {
		t.Errorf("202 model = %#v", rerr.Model)
	}

	status, ctype, body = 200, "application/x-ndjson", "{\"i\":0}\n{\"i\":1}\n"
	if s, err := api.StreamLines(t.Context()); err != nil || s != body {
		t.Errorf("ndjson: %#v, %v", s, err)
	}

	status, ctype, body = 200, "application/json", "null"
	if o, err := api.LatestOrder(t.Context()); err != nil || o != nil {
		t.Errorf("null: %+v, %v", o, err)
	}

	// A nullable string sent as text stays a string.
	status, ctype, body = 200, "text/plain", "hello"
	if s, err := api.Greeting(t.Context()); err != nil || s != "hello" {
		t.Errorf("text: %#v, %v", s, err)
	}

	// An any result holds every 2xx body.
	status, ctype, body = 201, "application/json", `{"id":1}`
	if v, err := api.Anything(t.Context()); err != nil || v == nil {
		t.Errorf("any 201: %#v, %v", v, err)
	}
}
