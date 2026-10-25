package parameterspathstyle

import (
	"context"
	"net/http"
	"testing"
)

func TestPathStyles(t *testing.T) {
	api := NewDefaultAPI(nil)
	ctx := context.Background()
	ids := []string{"a", "b"}

	for want, build := range map[string]func() (*http.Request, error){
		"/simple/a,b":     func() (*http.Request, error) { return api.GetSimplePathRequest(ctx, GetSimplePathParams{ID: ids}) },
		"/matrix/;id=a,b": func() (*http.Request, error) { return api.GetMatrixPathRequest(ctx, GetMatrixPathParams{ID: ids}) },
		"/label/.a.b":     func() (*http.Request, error) { return api.GetLabelPathRequest(ctx, GetLabelPathParams{ID: ids}) },
	} {
		req, err := build()
		if err != nil {
			t.Fatal(err)
		}

		if got := req.URL.EscapedPath(); got != want {
			t.Errorf("path = %s, want %s", got, want)
		}
	}
}
