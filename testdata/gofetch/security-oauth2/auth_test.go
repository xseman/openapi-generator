package securityoauth2

import (
	"context"
	"reflect"
	"testing"
)

func TestOAuth2Scopes(t *testing.T) {
	var got []string

	api := NewDefaultAPI(&Configuration{
		AccessToken: func(_ context.Context, op, name string, scopes []string) (string, error) {
			got = append([]string{op, name}, scopes...)

			return "tok", nil
		},
	})

	req, err := api.GetWithOAuthCodeRequest(context.Background())
	if err != nil || req.Header.Get("Authorization") != "Bearer tok" {
		t.Errorf("oauth2: %v, %v", req.Header, err)
	}

	if want := []string{"getWithOAuthCode", "OAuth2Code", "read", "write"}; !reflect.DeepEqual(got, want) {
		t.Errorf("AccessToken called with %q, want %q", got, want)
	}
}
