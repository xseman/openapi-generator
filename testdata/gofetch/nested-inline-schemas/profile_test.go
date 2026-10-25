package nestedinlineschemas

import (
	"encoding/json"
	"testing"
)

// TestInlineObjects checks that inline objects decode into their declared
// types, nested and as slice elements, and encode back unchanged.
func TestInlineObjects(t *testing.T) {
	const in = `{"user":{"name":"ann","address":{"street":"Main","geo":{"lat":1.5,"lng":-2}}},"settings":[{"key":"k","value":{"raw":"r"}}]}`

	var p Profile
	if err := json.Unmarshal([]byte(in), &p); err != nil {
		t.Fatal(err)
	}

	var geo *ProfileUserAddressGeo = p.User.Address.Geo
	if *geo.Lat != 1.5 || *p.Settings[0].Value.Raw != "r" {
		t.Errorf("decoded %+v", p)
	}

	out, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}

	if string(out) != in {
		t.Errorf("round trip = %s", out)
	}
}
