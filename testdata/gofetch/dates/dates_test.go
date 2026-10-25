package dates

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestDates(t *testing.T) {
	var query url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"startDate":"2024-01-02","createdAt":"2024-01-02T03:04:05Z","reminderDates":["2024-01-01"]}]`))
	}))
	defer srv.Close()

	at := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	api := NewDefaultAPI(&Configuration{BasePath: srv.URL})

	events, err := api.ListEvents(context.Background(), ListEventsParams{Since: &at, OnDay: Date{at}})
	if err != nil {
		t.Fatal(err)
	}

	if query.Get("since") != "2024-01-02T03:04:05Z" || query.Get("onDay") != "2024-01-02" {
		t.Errorf("query = %v", query)
	}

	e := events[0]
	if e.StartDate.String() != "2024-01-02" || !e.CreatedAt.Equal(at) || e.EndDate.Set || e.ReminderDates[0].String() != "2024-01-01" {
		t.Errorf("event = %+v", e)
	}

	out, err := json.Marshal(Event{StartDate: e.StartDate, CreatedAt: at, EndDate: Some(Date{at})})
	if err != nil {
		t.Fatal(err)
	}

	if want := `{"startDate":"2024-01-02","endDate":"2024-01-02","createdAt":"2024-01-02T03:04:05Z"}`; string(out) != want {
		t.Errorf("Marshal = %s, want %s", out, want)
	}
}
