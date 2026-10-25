package gen

import (
	"testing"

	"github.com/xseman/openapi-generator/internal/generator/typescript"
)

// TestResolveSecurity checks that an operation's auth methods carry their
// scheme's type and key, and that one without security of its own takes the
// document's while "security: []" opts out.
func TestResolveSecurity(t *testing.T) {
	spec := writeSpec(t, "spec.yaml", `openapi: 3.0.0
info: {title: t, version: "1"}
security: [{api_key: []}]
paths:
  /inherits:
    get:
      operationId: inherits
      responses: {"204": {description: ok}}
  /none:
    get:
      operationId: none
      security: []
      responses: {"204": {description: ok}}
  /own:
    get:
      operationId: own
      security: [{oauth: [read]}]
      responses: {"204": {description: ok}}
components:
  securitySchemes:
    api_key: {type: apiKey, in: header, name: X-API-Key}
    oauth:
      type: oauth2
      flows: {implicit: {authorizationUrl: "https://example.com", scopes: {read: r, write: w}}}
`)

	data, err := assembleSpec(typescript.NewFetchGenerator(), Options{InputSpec: spec})
	if err != nil {
		t.Fatal(err)
	}

	ops := map[string]int{}
	for _, op := range data.operationsByTag["default"] {
		ops[op.OperationId]++

		switch op.OperationId {
		case "inherits":
			if len(op.AuthMethods) != 1 || !op.AuthMethods[0].IsApiKey || !op.AuthMethods[0].IsKeyInHeader || op.AuthMethods[0].KeyParamName != "X-API-Key" {
				t.Errorf("inherits auth = %+v, want the api_key header scheme", op.AuthMethods)
			}

		case "none":
			if len(op.AuthMethods) != 0 || op.HasAuthMethods {
				t.Errorf("none auth = %+v, want none", op.AuthMethods)
			}

		case "own":
			if len(op.AuthMethods) != 1 || !op.AuthMethods[0].IsOAuth || len(op.AuthMethods[0].Scopes) != 1 || op.AuthMethods[0].Scopes[0]["scope"] != "read" {
				t.Errorf("own auth = %+v, want oauth with the operation's scope read", op.AuthMethods)
			}
		}
	}

	if len(ops) != 3 {
		t.Errorf("operations = %v, want inherits, none, own", ops)
	}
}
