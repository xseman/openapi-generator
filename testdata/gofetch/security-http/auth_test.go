package securityhttp

import (
	"context"
	"testing"
)

func TestHTTPAuth(t *testing.T) {
	var scheme string

	api := NewDefaultAPI(&Configuration{
		Username: "u",
		Password: "p",
		AccessToken: func(_ context.Context, op, name string, scopes []string) (string, error) {
			scheme = op + " " + name

			return "tok", nil
		},
	})
	ctx := context.Background()

	req, err := api.GetWithBasicAuthRequest(ctx)
	if user, pass, ok := req.BasicAuth(); err != nil || !ok || user != "u" || pass != "p" {
		t.Errorf("basic auth = %q %q %v, %v", user, pass, ok, err)
	}

	req, err = api.GetWithBearerAuthRequest(ctx)
	if err != nil || req.Header.Get("Authorization") != "Bearer tok" {
		t.Errorf("bearer: %v, %v", req.Header, err)
	}

	if scheme != "getWithBearerAuth BearerAuth" {
		t.Errorf("AccessToken called with %q", scheme)
	}
}
