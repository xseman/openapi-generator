package requestbodymultipart

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestMultipartBody(t *testing.T) {
	req, err := NewDefaultAPI(nil).UploadFileRequest(context.Background(), UploadFileParams{
		File:        File{Name: "a.txt", Body: strings.NewReader("hello")},
		Attachments: []File{{Name: "b.txt", Body: strings.NewReader("b")}, {Name: "c.txt", Body: strings.NewReader("c")}},
		Description: Ptr("two files"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(req.Header.Get("Content-Type"), "multipart/form-data; boundary=") {
		t.Errorf("Content-Type = %q", req.Header.Get("Content-Type"))
	}

	if err := req.ParseMultipartForm(1 << 20); err != nil {
		t.Fatal(err)
	}

	form := req.MultipartForm
	if got := form.Value["description"]; len(got) != 1 || got[0] != "two files" {
		t.Errorf("description = %q", got)
	}

	if got := form.File["attachments"]; len(got) != 2 || got[0].Filename != "b.txt" || got[1].Filename != "c.txt" {
		t.Errorf("attachments = %v", got)
	}

	files := form.File["file"]
	if len(files) != 1 || files[0].Filename != "a.txt" {
		t.Fatalf("file = %v", files)
	}

	f, err := files[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if data, _ := io.ReadAll(f); string(data) != "hello" {
		t.Errorf("file content = %q", data)
	}
}
