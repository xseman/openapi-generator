package compositiononeof

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestOneOfRoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body) // echo
	}))
	defer srv.Close()

	var in SearchResult
	if err := in.FromCat(Cat{PetType: "cat", HuntingSkill: "lazy"}); err != nil {
		t.Fatal(err)
	}

	out, err := NewDefaultAPI(&Configuration{BasePath: srv.URL}).Search(context.Background(), SearchParams{SearchResult: in})
	if err != nil {
		t.Fatal(err)
	}

	if cat, err := out.AsCat(); err != nil || cat.HuntingSkill != "lazy" {
		t.Errorf("AsCat = %+v, %v", cat, err)
	}

	var list SearchResult
	if err := json.Unmarshal([]byte(`[1,2]`), &list); err != nil {
		t.Fatal(err)
	}

	if ints, err := list.AsIntList(); err != nil || !reflect.DeepEqual(ints, []int{1, 2}) {
		t.Errorf("AsIntList = %v, %v", ints, err)
	}

	if _, err := list.AsString(); err == nil {
		t.Error("AsString of a list succeeded")
	}

	if b, err := json.Marshal(SearchResult{}); err != nil || string(b) != "null" {
		t.Errorf("empty union = %s, %v", b, err)
	}
}
