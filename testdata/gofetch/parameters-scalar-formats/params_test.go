package parametersscalarformats

import (
	"testing"
	"time"
)

// TestScalarText sends each scalar as its shortest exact text: no float32
// noise, no exponent, int64 above 2^53, RFC 3339 times, base64 text escaped.
func TestScalarText(t *testing.T) {
	at := time.Date(2024, 2, 29, 23, 59, 58, 500, time.FixedZone("", -90*60))

	req, err := NewDefaultAPI(nil).SearchRequest(t.Context(), SearchParams{
		F32: Ptr(float32(0.1)), F64: Ptr(1e21), N: Ptr(0.30000000000000004), I64: Ptr(int64(1<<62 + 1)), Ok: Ptr(false),
		Day: &Date{Time: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)}, At: &DateTime{Time: at}, Raw: Ptr("+/8="),
	})
	if err != nil {
		t.Fatal(err)
	}

	want := "at=2024-02-29T23%3A59%3A58.0000005-01%3A30&day=2024-01-02&f32=0.1&f64=1000000000000000000000" +
		"&i64=4611686018427387905&n=0.30000000000000004&ok=false&raw=%2B%2F8%3D"
	if got := req.URL.RawQuery; got != want {
		t.Errorf("query = %s\nwant    %s", got, want)
	}
}
