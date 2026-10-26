package responsesoneof

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOneOfResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"accountType":"business","companyName":"Acme"}`))
	}))
	defer srv.Close()

	acc, err := NewDefaultAPI(&Configuration{BasePath: srv.URL}).GetAccount(context.Background(), GetAccountParams{ID: "1"})
	if err != nil {
		t.Fatal(err)
	}

	if b, err := acc.AsBusinessAccount(); err != nil || b.CompanyName != "Acme" {
		t.Errorf("AsBusinessAccount = %+v, %v", b, err)
	}

	// A business account lacks ownerName, which a personal one requires.
	if p, err := acc.AsPersonalAccount(); err == nil {
		t.Errorf("AsPersonalAccount = %+v, want an error", p)
	}
}
