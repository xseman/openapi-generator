package nullableopenapi31

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestNullMembersTyped decodes the 3.1 spellings of null into the member's
// type and encodes them back unchanged.
func TestNullMembersTyped(t *testing.T) {
	const in = `{"kind":"pet","name":"Mia","size":null,"owner":{"name":"Ann"},"born":"2020-05-17","nickname":null,"tags":["a"]}`

	var p Pet
	if err := json.Unmarshal([]byte(in), &p); err != nil {
		t.Fatal(err)
	}

	// A component that is a nullable string is one wherever it is referred to.
	var alias Opt[string] = p.Alias

	if alias.Set || p.Kind != PetKindEnumPet || p.Size != nil || p.Owner.Value.Name != "Ann" || p.Born.Value.Day() != 17 || !p.Nickname.Null || p.Tags.Value[0] != "a" {
		t.Errorf("decoded %+v", p)
	}

	p.Size = Ptr(Size2)

	out, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}

	if want := strings.Replace(in, `"size":null`, `"size":2`, 1); string(out) != want {
		t.Errorf("encoded %s\nwant    %s", out, want)
	}
}

type recorder struct{ req *http.Request }

func (r *recorder) Do(req *http.Request) (*http.Response, error) {
	r.req = req

	return &http.Response{StatusCode: 204, Body: http.NoBody, Header: http.Header{}}, nil
}

// TestContentMediaTypeIsFile sends a contentMediaType string as a file part.
func TestContentMediaTypeIsFile(t *testing.T) {
	rec := &recorder{}
	api := NewDefaultAPI(&Configuration{BasePath: "http://x", HTTPClient: rec})

	err := api.UploadPhoto(t.Context(), UploadPhotoParams{
		ID: "1", File: File{Name: "a.png", Body: strings.NewReader("png")},
		Tags: []string{"a", "b"}, Labels: []string{"x", "y"},
	})
	if err != nil {
		t.Fatal(err)
	}

	mr, err := rec.req.MultipartReader()
	if err != nil {
		t.Fatal(err)
	}

	part, err := mr.NextPart()
	if err != nil {
		t.Fatal(err)
	}

	body, _ := io.ReadAll(part)
	if part.FormName() != "file" || part.FileName() != "a.png" || string(body) != "png" {
		t.Errorf("part %s %q = %q", part.FormName(), part.FileName(), body)
	}

	// An array is a part per item, one comma-joined part with explode: false.
	form, err := mr.ReadForm(1 << 20)
	if err != nil {
		t.Fatal(err)
	}

	if got := fmt.Sprint(form.Value); got != "map[labels:[x,y] tags:[a b]]" {
		t.Errorf("fields = %s", got)
	}
}
