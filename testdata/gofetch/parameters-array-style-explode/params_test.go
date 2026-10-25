package parametersarraystyleexplode

import (
	"context"
	"reflect"
	"testing"
)

func TestArrayStyles(t *testing.T) {
	req, err := NewDefaultAPI(nil).FilterItemsRequest(context.Background(), FilterItemsParams{
		FormExplodeTrue:  []string{"a", "b"},
		FormExplodeFalse: []string{"a", "b"},
		SpaceDelimited:   []int{1, 2},
		PipeDelimited:    []int{1, 2},
		PathIds:          []string{"x y", "z/w"},
	})
	if err != nil {
		t.Fatal(err)
	}

	q := req.URL.Query()
	for name, want := range map[string][]string{
		"formExplodeTrue":  {"a", "b"},
		"formExplodeFalse": {"a,b"},
		"spaceDelimited":   {"1 2"},
		"pipeDelimited":    {"1|2"},
	} {
		if !reflect.DeepEqual(q[name], want) {
			t.Errorf("%s = %q, want %q", name, q[name], want)
		}
	}

	if got, want := req.URL.EscapedPath(), "/filter/x%20y,z%2Fw"; got != want {
		t.Errorf("path = %s, want %s", got, want)
	}
}

func TestEmptyArrayIsLeftOut(t *testing.T) {
	req, err := NewDefaultAPI(nil).FilterItemsRequest(context.Background(), FilterItemsParams{PathIds: []string{"x"}})
	if err != nil {
		t.Fatal(err)
	}

	if req.URL.RawQuery != "" {
		t.Errorf("query = %q, want none", req.URL.RawQuery)
	}
}
