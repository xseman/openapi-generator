package additionalpropertieswithproperties

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestExtraKeys reads undeclared keys into AdditionalProperties and writes
// them back after the declared fields, typed by the schema.
func TestExtraKeys(t *testing.T) {
	const in = `{"kind":"click","x":1,"tags":["a"]}`

	var e Event
	if err := json.Unmarshal([]byte(in), &e); err != nil {
		t.Fatal(err)
	}

	if want := map[string]any{"x": 1.0, "tags": []any{"a"}}; e.Kind != "click" || !reflect.DeepEqual(e.AdditionalProperties, want) {
		t.Errorf("decoded %#v", e)
	}

	out, err := json.Marshal(e)
	if err != nil || string(out) != `{"kind":"click","tags":["a"],"x":1}` {
		t.Errorf("encoded %s, %v", out, err)
	}

	// A declared key in the map does not shadow the field.
	s := Scores{Player: "p", AdditionalProperties: map[string]int{"player": 9, "round1": 3}}
	if out, _ := json.Marshal(s); string(out) != `{"player":"p","round1":3}` {
		t.Errorf("encoded %s", out)
	}

	if err := json.Unmarshal([]byte(`{"player":"p","bad":"x"}`), &s); err == nil {
		t.Error("a string decoded as an int score")
	}

	if out, _ := json.Marshal(Event{Kind: "k"}); string(out) != `{"kind":"k"}` {
		t.Errorf("no extras: %s", out)
	}

	// Extras only, the declared fields all left out.
	if out, _ := json.Marshal(Notes{AdditionalProperties: map[string]string{"b": "2", "a": "1"}}); string(out) != `{"a":"1","b":"2"}` {
		t.Errorf("extras only: %s", out)
	}
}
