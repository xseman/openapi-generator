package responsesstream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func serve(t *testing.T, contentType, body string) *DefaultAPI {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	return NewDefaultAPI(&Configuration{BasePath: srv.URL})
}

func TestJSONLines(t *testing.T) {
	ticks, err := serve(t, "application/jsonl", "{\"i\":0,\"label\":\"a\"}\n{\"i\":1}\n").StreamTicks(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	var got []Tick

	for tick, err := range ticks {
		if err != nil {
			t.Fatal(err)
		}

		got = append(got, tick)
	}

	if len(got) != 2 || got[0].I != 0 || *got[0].Label != "a" || got[1].I != 1 {
		t.Errorf("ticks = %+v", got)
	}

	// A bad line ends the range with its error; a break stops it early.
	words, _ := serve(t, "application/jsonl", "\"a\"\n\"b\"\nnot json\n\"c\"\n").StreamWords(context.Background())

	var n int

	for _, err := range words {
		if n++; err != nil {
			break
		}
	}

	if n != 3 {
		t.Errorf("words read until the error = %d, want 3", n)
	}

	for range words {
		break
	}
}

func TestServerSentEvents(t *testing.T) {
	body := ": ping\n\nevent: delta\nid: 1\ndata: {\"kind\":\"delta\",\n" +
		"data: \"text\":\"hi\"}\n\nretry: 500\ndata: {\"kind\":\"done\",\"words\":1}\n\n"

	events, err := serve(t, "text/event-stream", body).Chat(context.Background(), ChatParams{Body: ChatBody{Prompt: "hi"}})
	if err != nil {
		t.Fatal(err)
	}

	var got []ServerSentEvent[ChatItem]

	for e, err := range events {
		if err != nil {
			t.Fatal(err)
		}

		got = append(got, e)
	}

	if len(got) != 2 || got[0].Event != "delta" || got[0].ID != "1" || got[1].Retry != 500 {
		t.Fatalf("events = %+v", got)
	}

	if d, err := got[0].Data.AsDelta(); err != nil || d.Text != "hi" {
		t.Errorf("first data = %+v, %v", d, err)
	}

	if d, err := got[1].Data.AsDone(); err != nil || d.Words != 1 {
		t.Errorf("last data = %+v, %v", d, err)
	}

	// Data of a string schema is the text itself; an id holds until the next
	// one; a BOM is skipped; an event the body ends before is dropped.
	raw, _ := serve(t, "text/event-stream", "\ufeffid: 7\ndata: {\"i\":1}\n\nevent: end\ndata: bye\n\ndata: cut").RawEvents(context.Background())

	var texts []string

	for e, err := range raw {
		if err != nil {
			t.Fatal(err)
		}

		texts = append(texts, e.ID+e.Event+"="+e.Data)
	}

	if len(texts) != 2 || texts[0] != `7={"i":1}` || texts[1] != "7end=bye" {
		t.Errorf("raw events = %q", texts)
	}
}
