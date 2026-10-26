package requestbodyformencoding

import (
	"context"
	"io"
	"net/url"
	"reflect"
	"testing"
)

// TestFormEncoding checks each form field against its encoding: deepObject
// spells out nested objects and lists, an array explodes by default.
func TestFormEncoding(t *testing.T) {
	req, err := NewDefaultAPI(nil).CreateCustomerRequest(context.Background(), CreateCustomerParams{
		Name:     "ann",
		Address:  &CreateCustomerAddress{City: Ptr("Oslo"), Lines: []string{"a", "b"}},
		Metadata: map[string]string{"k": "v"},
		Expand:   []string{"x", "y"},
		Tags:     []string{"t", "u"},
		Ids:      []int{1, 2},
	})
	if err != nil {
		t.Fatal(err)
	}

	body, _ := io.ReadAll(req.Body)

	got, err := url.ParseQuery(string(body))
	if err != nil {
		t.Fatal(err)
	}

	want := url.Values{
		"name":              {"ann"},
		"address[city]":     {"Oslo"},
		"address[lines][0]": {"a"},
		"address[lines][1]": {"b"},
		"metadata[k]":       {"v"},
		"expand[0]":         {"x"},
		"expand[1]":         {"y"},
		"tags":              {"t,u"},
		"ids":               {"1", "2"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("form = %v\nwant   %v", got, want)
	}
}
