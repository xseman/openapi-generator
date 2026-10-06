package parser

import "testing"

// TestDateTypeFollowsMappedType checks that IsDateType/IsDateTimeType mark only a
// date mapped to a JS Date. withoutRuntimeChecks maps dates to string, and the
// templates must then pass the value through instead of calling the runtime's
// serializeDate on a string.
func TestDateTypeFollowsMappedType(t *testing.T) {
	spec := []byte(`
openapi: 3.0.0
info:
  title: Dates
  version: 1.0.0
paths:
  /events:
    get:
      operationId: listEvents
      parameters:
        - name: onDay
          in: query
          schema:
            type: string
            format: date
      responses:
        '200':
          description: ok
components:
  schemas:
    Event:
      type: object
      properties:
        createdAt:
          type: string
          format: date-time
`)

	for _, tt := range []struct {
		mapped string
		want   bool
	}{
		{"Date", true},
		{"string", false},
	} {
		p := NewParser()
		p.SkipValidation = true
		p.GetTypeFunc = func(schemaType, format string) string {
			if format == "date" || format == "date-time" {
				return tt.mapped
			}

			return schemaType
		}

		if err := p.LoadFromData(spec); err != nil {
			t.Fatalf("LoadFromData: %v", err)
		}

		models, err := p.GetModels()
		if err != nil {
			t.Fatalf("GetModels: %v", err)
		}

		if got := findModel(t, models, "Event").Vars[0].IsDateTimeType; got != tt.want {
			t.Errorf("mapped %s: createdAt.IsDateTimeType = %v, want %v", tt.mapped, got, tt.want)
		}

		ops, err := p.GetOperations()
		if err != nil {
			t.Fatalf("GetOperations: %v", err)
		}

		for _, list := range ops {
			for _, op := range list {
				if got := op.QueryParams[0].IsDateType; got != tt.want {
					t.Errorf("mapped %s: onDay.IsDateType = %v, want %v", tt.mapped, got, tt.want)
				}
			}
		}
	}
}
