package securityapikey

import (
	"context"
	"testing"
)

func TestAPIKey(t *testing.T) {
	var calls []string

	api := NewDefaultAPI(&Configuration{
		APIKey: func(_ context.Context, op, name string) (string, error) {
			calls = append(calls, op+" "+name)

			return "k-" + name, nil
		},
	})
	ctx := context.Background()

	req, err := api.GetWithHeaderKeyRequest(ctx)
	if err != nil || req.Header.Get("X-Api-Key") != "k-X-Api-Key" {
		t.Errorf("header key: %v, %v", req.Header, err)
	}

	req, err = api.GetWithQueryKeyRequest(ctx)
	if err != nil || req.URL.Query().Get("api_key") != "k-api_key" {
		t.Errorf("query key: %v, %v", req.URL, err)
	}

	req, err = api.GetWithCookieKeyRequest(ctx)
	if c, cerr := req.Cookie("session"); err != nil || cerr != nil || c.Value != "k-session" {
		t.Errorf("cookie key: %v, %v, %v", c, err, cerr)
	}

	want := []string{"getWithHeaderKey X-Api-Key", "getWithQueryKey api_key", "getWithCookieKey session"}
	if len(calls) != len(want) {
		t.Fatalf("calls = %q, want %q", calls, want)
	}

	for i := range want {
		if calls[i] != want[i] {
			t.Errorf("call %d = %q, want %q", i, calls[i], want[i])
		}
	}
}

func TestNoKeyConfigured(t *testing.T) {
	req, err := NewDefaultAPI(nil).GetWithHeaderKeyRequest(context.Background())
	if err != nil || req.Header.Get("X-Api-Key") != "" {
		t.Errorf("header = %v, err %v, want no key", req.Header, err)
	}
}
