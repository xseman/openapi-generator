package requestbodyformurlencoded

import (
	"context"
	"io"
	"net/url"
	"testing"
)

func TestFormBody(t *testing.T) {
	req, err := NewDefaultAPI(nil).LoginRequest(context.Background(), LoginParams{
		Username:   "ann",
		Password:   "s3cret&",
		RememberMe: Ptr(true),
		Tags:       []string{"a", "b"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if ct := req.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q", ct)
	}

	body, _ := io.ReadAll(req.Body)

	form, err := url.ParseQuery(string(body))
	if err != nil {
		t.Fatal(err)
	}

	for name, want := range map[string]string{"username": "ann", "password": "s3cret&", "rememberMe": "true"} {
		if got := form.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}

	// An array is exploded by default, one key per value.
	if got := form["tags"]; len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("tags = %q, want [a b]", got)
	}
}
