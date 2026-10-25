package requestbodyformurlencoded

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/url"
	"strings"
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

// TestFormWithFile: only multipart carries a file, so a form declared
// url-encoded goes as multipart once it holds one.
func TestFormWithFile(t *testing.T) {
	req, err := NewDefaultAPI(nil).UploadAvatarRequest(context.Background(), UploadAvatarParams{
		User:  "ann",
		Image: File{Name: "a.png", Body: strings.NewReader("PNG")},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil {
		t.Fatal(err)
	}

	form, err := multipart.NewReader(req.Body, params["boundary"]).ReadForm(1 << 20)
	if err != nil {
		t.Fatalf("Content-Type %q: %v", req.Header.Get("Content-Type"), err)
	}

	if got := form.Value["user"]; len(got) != 1 || got[0] != "ann" {
		t.Errorf("user = %q", got)
	}

	if f := form.File["image"]; len(f) != 1 || f[0].Filename != "a.png" || f[0].Size != 3 {
		t.Errorf("image = %+v", f)
	}
}
