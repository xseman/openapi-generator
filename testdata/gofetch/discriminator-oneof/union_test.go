package discriminatoroneof

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestValueByDiscriminator(t *testing.T) {
	for _, tt := range []struct {
		json, want string
	}{
		{`{"petType":"cat","huntingSkill":"lazy"}`, "*discriminatoroneof.Cat"},
		{`{"petType":"dog","packSize":3}`, "*discriminatoroneof.Dog"},
		{`{"petType":"canine","packSize":3}`, "*discriminatoroneof.Dog"},
	} {
		var p Pet
		if err := json.Unmarshal([]byte(tt.json), &p); err != nil {
			t.Fatal(err)
		}

		v, err := p.ValueByDiscriminator()
		if got := fmt.Sprintf("%T", v); err != nil || got != tt.want {
			t.Errorf("%s: ValueByDiscriminator = %s, %v, want %s", tt.json, got, err, tt.want)
		}
	}

	var p Pet
	if err := json.Unmarshal([]byte(`{"petType":"bird"}`), &p); err != nil {
		t.Fatal(err)
	}

	if d, err := p.Discriminator(); err != nil || d != "bird" {
		t.Errorf("Discriminator = %q, %v", d, err)
	}

	if v, err := p.ValueByDiscriminator(); err == nil {
		t.Errorf("unknown petType decoded to %v", v)
	}
}
