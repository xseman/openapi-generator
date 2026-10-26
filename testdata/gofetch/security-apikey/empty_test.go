package securityapikey

import (
	"context"
	"errors"
	"testing"
)

// TestEmptyOrFailingKey sends no key for an empty one and stops on an error.
func TestEmptyOrFailingKey(t *testing.T) {
	empty := NewDefaultAPI(&Configuration{APIKey: func(context.Context, string, string) (string, error) { return "", nil }})

	req, err := empty.GetWithQueryKeyRequest(t.Context())
	if err != nil || req.URL.RawQuery != "" {
		t.Errorf("empty key: %q, %v", req.URL.RawQuery, err)
	}

	boom := errors.New("vault down")
	failing := NewDefaultAPI(&Configuration{APIKey: func(context.Context, string, string) (string, error) { return "", boom }})

	if _, err := failing.GetWithHeaderKeyRequest(t.Context()); !errors.Is(err, boom) {
		t.Errorf("failing key: %v", err)
	}
}
