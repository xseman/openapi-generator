package responsesbinary

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBinary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte{0, 1, 2})
	}))
	defer srv.Close()

	api := NewDefaultAPI(&Configuration{BasePath: srv.URL})

	data, err := api.DownloadFile(context.Background(), DownloadFileParams{ID: "1"})
	if err != nil || string(data) != "\x00\x01\x02" {
		t.Errorf("DownloadFile = %v, %v", data, err)
	}

	resp, err := api.GetThumbnailRaw(context.Background(), GetThumbnailParams{ID: "1"})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if data, _ := io.ReadAll(resp.Body); len(data) != 3 {
		t.Errorf("GetThumbnailRaw body = %v", data)
	}
}
