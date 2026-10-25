package compositiononeofinline

import (
	"encoding/json"
	"io"
	"testing"
)

// TestInlineUnion decodes inline unions into their variants and encodes
// them back unchanged.
func TestInlineUnion(t *testing.T) {
	const in = `{"favourite":{"kind":"cat","lives":3},"residents":[{"kind":"dog","barkVolume":2}]}`

	var s Shelter
	if err := json.Unmarshal([]byte(in), &s); err != nil {
		t.Fatal(err)
	}

	if cat, err := s.Favourite.AsCat(); err != nil || cat.Lives != 3 {
		t.Errorf("favourite AsCat = %+v, %v", cat, err)
	}

	if _, err := s.Favourite.AsDog(); err == nil {
		t.Error("favourite AsDog of a cat succeeded")
	}

	if dog, err := s.Residents[0].AsDog(); err != nil || dog.BarkVolume != 2 {
		t.Errorf("resident AsDog = %+v, %v", dog, err)
	}

	out, err := json.Marshal(s)
	if err != nil || string(out) != in {
		t.Errorf("encoded %s, %v\nwant    %s", out, err, in)
	}
}

// TestInlineUnionBody sends the variant set with From as the body.
func TestInlineUnionBody(t *testing.T) {
	var body AdmitBody
	if err := body.FromDog(Dog{Kind: "dog", BarkVolume: 1.5}); err != nil {
		t.Fatal(err)
	}

	req, err := NewDefaultAPI(&Configuration{BasePath: "http://x"}).AdmitRequest(t.Context(), AdmitParams{ID: "s1", Body: body})
	if err != nil {
		t.Fatal(err)
	}

	got, _ := io.ReadAll(req.Body)
	if want := `{"kind":"dog","barkVolume":1.5}`; string(got) != want {
		t.Errorf("body %s, want %s", got, want)
	}
}

// TestImpliedDiscriminator fills in and checks the kind each variant pins,
// and dispatches on it.
func TestImpliedDiscriminator(t *testing.T) {
	var f ShelterFavourite
	if err := f.FromDog(Dog{BarkVolume: 3}); err != nil {
		t.Fatal(err)
	}

	if b, _ := json.Marshal(f); string(b) != `{"kind":"dog","barkVolume":3}` {
		t.Errorf("FromDog = %s", b)
	}

	// Every key a Cat requires is there, but the kind says dog.
	_ = json.Unmarshal([]byte(`{"kind":"dog","lives":1,"barkVolume":3}`), &f)
	if _, err := f.AsCat(); err == nil {
		t.Error("AsCat of a dog succeeded")
	}

	v, err := f.ValueByDiscriminator()
	if dog, ok := v.(*Dog); err != nil || !ok || dog.BarkVolume != 3 {
		t.Errorf("ValueByDiscriminator = %#v, %v", v, err)
	}
}

// TestInlineObjectVariants gives each inline object member a struct, told
// apart by its required properties.
func TestInlineObjectVariants(t *testing.T) {
	var s Shelter
	if err := json.Unmarshal([]byte(`{"favourite":{"kind":"cat","lives":1},"location":{"lat":1.5,"lon":2}}`), &s); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Location.AsShelterLocationOneOf(); err == nil {
		t.Error("a point read as a street")
	}

	if p, err := s.Location.AsShelterLocationOneOf1(); err != nil || p.Lat != 1.5 || p.Lon != 2 {
		t.Errorf("AsShelterLocationOneOf1 = %+v, %v", p, err)
	}
}
