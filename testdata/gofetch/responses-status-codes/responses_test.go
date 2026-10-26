package responsesstatuscodes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatusCodes(t *testing.T) {
	status, body := 0, ""

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	api := NewDefaultAPI(&Configuration{BasePath: srv.URL})
	ctx := context.Background()

	status, body = 200, `{"id":"1","title":"t"}`
	if task, err := api.GetTask(ctx, GetTaskParams{ID: "1"}); err != nil || *task.Title != "t" {
		t.Errorf("200: %+v, %v", task, err)
	}

	for _, code := range []int{404, 500} {
		status, body = code, `{"message":"nope"}`
		_, err := api.GetTask(ctx, GetTaskParams{ID: "1"})

		var rerr *ResponseError
		if !errors.As(err, &rerr) || rerr.StatusCode != code || string(rerr.Body) != body {
			t.Fatalf("%d: error %v", code, err)
		}

		if m, ok := rerr.Model.(*ErrorBody); !ok || *m.Message != "nope" {
			t.Errorf("%d: model = %#v", code, rerr.Model)
		}
	}

	status, body = 204, ""
	if err := api.DeleteTask(ctx, DeleteTaskParams{ID: "1"}); err != nil {
		t.Errorf("204: %v", err)
	}

	status, body = 201, `{"id":"1"}`
	if task, err := api.CreateTask(ctx, CreateTaskParams{ID: "1"}); err != nil || task == nil || *task.ID != "1" {
		t.Errorf("201: %+v, %v", task, err)
	}

	status, body = 202, ""
	if task, err := api.CreateTask(ctx, CreateTaskParams{ID: "1"}); err != nil || task != nil {
		t.Errorf("202 without a body: %+v, %v", task, err)
	}

	status, body = 409, "conflict"
	if err := api.DeleteTask(ctx, DeleteTaskParams{ID: "1"}); err == nil || err.Error() != "409 Conflict" {
		t.Errorf("409: %v", err)
	}
}
