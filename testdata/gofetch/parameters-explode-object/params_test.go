package parametersexplodeobject

import (
	"context"
	"testing"
)

func TestExplodedObjects(t *testing.T) {
	req, err := NewDefaultAPI(nil).ListItemsRequest(context.Background(), ListItemsParams{
		Labels: map[string]string{"color": "red", "size": "L"},
		Extra:  map[string]any{"debug": true},
		Range:  map[string]int{"min": 1, "max": 5},
	})
	if err != nil {
		t.Fatal(err)
	}

	q := req.URL.Query()
	for name, want := range map[string]string{
		"color":      "red",
		"size":       "L",
		"debug":      "true",
		"range[min]": "1",
		"range[max]": "5",
	} {
		if got := q.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}

	if len(q) != 5 {
		t.Errorf("query = %v, want 5 parameters", q)
	}
}
