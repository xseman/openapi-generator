package parameterslocations

import (
	"context"
	"testing"
)

func TestLocations(t *testing.T) {
	api := NewDefaultAPI(nil)

	req, err := api.GetItemRequest(context.Background(), GetItemParams{
		OrgID:      "acme corp",
		ItemID:     7,
		Verbose:    Ptr(true),
		XRequestID: Ptr("req-1"),
		SessionID:  Ptr("s1"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := req.URL.EscapedPath(); got != "/orgs/acme%20corp/items/7" {
		t.Errorf("path = %s", got)
	}

	if got := req.URL.Query().Get("verbose"); got != "true" {
		t.Errorf("verbose = %q", got)
	}

	if got := req.Header.Get("X-Request-Id"); got != "req-1" {
		t.Errorf("X-Request-Id = %q", got)
	}

	if c, err := req.Cookie("sessionId"); err != nil || c.Value != "s1" {
		t.Errorf("sessionId cookie = %v, %v", c, err)
	}

	// Optional parameters left nil are not sent.
	req, err = api.GetItemRequest(context.Background(), GetItemParams{OrgID: "o", ItemID: 1})
	if err != nil {
		t.Fatal(err)
	}

	if req.URL.RawQuery != "" || req.Header.Get("X-Request-Id") != "" || len(req.Cookies()) != 0 {
		t.Errorf("unset parameters sent: query %q, header %v", req.URL.RawQuery, req.Header)
	}
}
