package parametersobjectstyles

import "testing"

// TestObjectStyles checks each style against the OpenAPI serialization
// table, for color = {R: 100, G: 200, B: 150} and the array [blue, black].
func TestObjectStyles(t *testing.T) {
	color := Color{R: 100, G: 200, B: 150}

	req, err := NewDefaultAPI(nil).PaintRequest(t.Context(), PaintParams{
		A: color, B: color, C: color, D: color, E: color, F: color, G: []string{"blue", "black"},
		Q: &color, XColor: &color, XColorExploded: &color, XTags: []string{"x", "y"}, Prefs: []string{"p", "q"},
	})
	if err != nil {
		t.Fatal(err)
	}

	want := "/simple/B,150,G,200,R,100/B=150,G=200,R=100" +
		"/label/.B,150,G,200,R,100/.B=150.G=200.R=100" +
		"/matrix/;e=B,150,G,200,R,100/;B=150;G=200;R=100/;g=blue;g=black"
	if got := req.URL.EscapedPath(); got != want {
		t.Errorf("path = %s\nwant   %s", got, want)
	}

	if got := req.URL.RawQuery; got != "q=B%2C150%2CG%2C200%2CR%2C100" {
		t.Errorf("query = %s", got)
	}

	// net/http quotes a cookie value with a comma, as RFC 6265 has no comma
	// in a bare cookie value.
	for name, want := range map[string]string{
		"X-Color": "B,150,G,200,R,100", "X-Color-Exploded": "B=150,G=200,R=100", "X-Tags": "x,y", "Cookie": `prefs="p,q"`,
	} {
		if got := req.Header.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}
