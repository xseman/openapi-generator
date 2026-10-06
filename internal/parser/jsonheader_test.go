package parser

import "testing"

// TestJSONHeaderUsesModelSerializer checks that a header parameter typed through a
// JSON content entry gets that entry's model type, and is marked for
// JSON.stringify(<Model>ToJSON(v)) instead of String(v).
func TestJSONHeaderUsesModelSerializer(t *testing.T) {
	spec := []byte(`
openapi: 3.0.0
info:
  title: JSON headers
  version: 1.0.0
paths:
  /json:
    get:
      operationId: getJson
      parameters:
        - name: X-Json-Arg
          in: header
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/HeaderArg'
        - name: X-Vendor-Arg
          in: header
          content:
            application/vnd.example+json; charset=utf-8:
              schema:
                $ref: '#/components/schemas/HeaderArg'
        - name: X-Text-Arg
          in: header
          content:
            text/plain:
              schema:
                $ref: '#/components/schemas/HeaderArg'
        - name: X-Plain-Arg
          in: header
          schema:
            type: string
      responses:
        '204':
          description: ok
components:
  schemas:
    HeaderArg:
      type: object
      properties:
        id:
          type: string
`)

	p := NewParser()

	p.SkipValidation = true
	if err := p.LoadFromData(spec); err != nil {
		t.Fatalf("LoadFromData: %v", err)
	}

	ops, err := p.GetOperations()
	if err != nil {
		t.Fatalf("GetOperations: %v", err)
	}

	want := map[string]bool{"X-Json-Arg": true, "X-Vendor-Arg": true, "X-Text-Arg": false, "X-Plain-Arg": false}

	for _, list := range ops {
		for _, op := range list {
			for _, hp := range op.HeaderParams {
				if got := hp.JsonHeaderUsesModelSerializer; got != want[hp.BaseName] {
					t.Errorf("%s.JsonHeaderUsesModelSerializer = %v, want %v", hp.BaseName, got, want[hp.BaseName])
				}

				if hp.BaseName != "X-Plain-Arg" && hp.DataType != "HeaderArg" {
					t.Errorf("%s.DataType = %q, want HeaderArg", hp.BaseName, hp.DataType)
				}
			}
		}
	}
}
