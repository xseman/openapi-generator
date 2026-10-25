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

	events, err := api.ListEvents(context.Background(), ListEventsParams{Since: &DateTime{Time: at}, OnDay: Date{at}})
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

	out, err := json.Marshal(Event{StartDate: e.StartDate, CreatedAt: DateTime{Time: at}, EndDate: Some(Date{at})})
	if err != nil {
		t.Fatal(err)
	}

	if want := `{"startDate":"2024-01-02","endDate":"2024-01-02","createdAt":"2024-01-02T03:04:05Z"}`; string(out) != want {
		t.Errorf("Marshal = %s, want %s", out, want)
	}
}

// TestNaiveDateTime reads a date-time without an offset, as FastAPI sends a
// naive datetime, and writes it back without one.
func TestNaiveDateTime(t *testing.T) {
	for in, want := range map[string]string{
		`"2024-03-01T10:00:00.500000"`: `"2024-03-01T10:00:00.500000"`,
		`"2024-03-01T10:00:00"`:        `"2024-03-01T10:00:00"`,
		`"2024-03-01T10:00:00.5"`:      `"2024-03-01T10:00:00.500000"`,
		`"2024-03-01T10:00:00+02:00"`:  `"2024-03-01T10:00:00+02:00"`,
	} {
		var d DateTime
		if err := json.Unmarshal([]byte(in), &d); err != nil {
			t.Fatalf("%s: %v", in, err)
		}

		if out, _ := json.Marshal(d); string(out) != want {
			t.Errorf("%s -> %s, want %s", in, out, want)
		}
	}

	var d DateTime
	if err := json.Unmarshal([]byte(`"yesterday"`), &d); err == nil {
		t.Error("yesterday decoded")
	}
}
