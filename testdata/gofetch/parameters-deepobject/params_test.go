package parametersdeepobject

import (
	"context"
	"testing"
)

func TestDeepObject(t *testing.T) {
	req, err := NewDefaultAPI(nil).DeepSearchRequest(context.Background(), DeepSearchParams{
		Filter: &DeepSearchFilter{Name: Ptr("cat food"), MinPrice: Ptr(1.5)},
	})
	if err != nil {
		t.Fatal(err)
	}

	q := req.URL.Query()
	if q.Get("filter[name]") != "cat food" || q.Get("filter[minPrice]") != "1.5" || len(q) != 2 {
		t.Errorf("query = %v", q)
	}
}
