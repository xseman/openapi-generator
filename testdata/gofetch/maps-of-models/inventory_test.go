package mapsofmodels

import (
	"encoding/json"
	"testing"
)

// TestEmptyContainerIsSent checks that an optional map or slice is left out
// only when nil: an empty one is a value, {} or [].
func TestEmptyContainerIsSent(t *testing.T) {
	tests := []struct {
		in   Inventory
		want string
	}{
		{Inventory{}, `{}`},
		{Inventory{ByWarehouse: map[string]Stock{}}, `{"byWarehouse":{}}`},
		{Inventory{HistoryByDate: map[string][]Stock{"d": {}}}, `{"historyByDate":{"d":[]}}`},
	}

	for _, tt := range tests {
		out, err := json.Marshal(tt.in)
		if err != nil {
			t.Fatal(err)
		}

		if string(out) != tt.want {
			t.Errorf("Marshal(%+v) = %s, want %s", tt.in, out, tt.want)
		}
	}
}
