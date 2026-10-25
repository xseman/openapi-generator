package requestbodyjson

import (
	"context"
	"io"
	"testing"
)

func TestJSONBodies(t *testing.T) {
	api := NewDefaultAPI(nil)
	ctx := context.Background()

	req, err := api.CreateItemRequest(ctx, CreateItemParams{Item: Item{Name: "pen"}})
	if err != nil {
		t.Fatal(err)
	}

	if body, _ := io.ReadAll(req.Body); string(body) != `{"name":"pen"}` || req.Header.Get("Content-Type") != "application/json" {
		t.Errorf("CreateItem body %s, content type %q", body, req.Header.Get("Content-Type"))
	}

	req, err = api.CreateItemsRequest(ctx, CreateItemsParams{Item: []Item{{Name: "a", Quantity: Ptr(2)}}})
	if err != nil {
		t.Fatal(err)
	}

	if body, _ := io.ReadAll(req.Body); string(body) != `[{"name":"a","quantity":2}]` {
		t.Errorf("CreateItems body %s", body)
	}

	// A nil slice is no body, not null.
	req, err = api.CreateItemsRequest(ctx, CreateItemsParams{})
	if err != nil || req.Body != nil {
		t.Errorf("CreateItems without items: body %v, %v", req.Body, err)
	}

	req, err = api.SetRawCountRequest(ctx, SetRawCountParams{Body: 3})
	if err != nil {
		t.Fatal(err)
	}

	if body, _ := io.ReadAll(req.Body); string(body) != `3` {
		t.Errorf("SetRawCount body %s", body)
	}
}
