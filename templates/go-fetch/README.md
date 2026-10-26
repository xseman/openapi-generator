# go-fetch

Generates a Go client in the shape of typescript-fetch: an API type per tag,
three methods per operation. The output imports only the standard library
and needs Go 1.24 (`omitzero`).

## Usage

```bash
openapi-generator generate -g go-fetch -i openapi.yaml -o ./petstore -p packageName=petstore
```

| Option        | Default       | Description                                  |
| ------------- | ------------- | -------------------------------------------- |
| `packageName` | `openapi`     | Go package name of the generated files       |
| `withGoMod`   | `false`       | write a `go.mod`, making the output a module |
| `moduleName`  | `packageName` | module path in `go.mod`                      |

## Output

| File              | Contents                                                         |
| ----------------- | ---------------------------------------------------------------- |
| `runtime.go`      | `Configuration`, `ResponseError`, `Opt`, `Date`, `File`, helpers |
| `api_<tag>.go`    | `<Tag>API`, a `<Operation>Params` per operation, its methods     |
| `model_<name>.go` | one model                                                        |
| `README.md`       | usage of the package                                             |
| `go.mod`          | with `withGoMod=true`                                            |

## The client

```go
cfg := &petstore.Configuration{
	HTTPClient:  &http.Client{Timeout: 10 * time.Second},
	AccessToken: func(ctx context.Context, op, name string, scopes []string) (string, error) { return token, nil },
}
pets := petstore.NewDefaultAPI(cfg)

pet, err := pets.FindPetByID(ctx, petstore.FindPetByIDParams{ID: 1})

var rerr *petstore.ResponseError
if errors.As(err, &rerr) {
	if e, ok := rerr.Model.(*petstore.Error); ok {
		log.Print(e.Message)
	}
}
```

| Method       | Does                                                                                                                       |
| ------------ | -------------------------------------------------------------------------------------------------------------------------- |
| `Xxx`        | sends and decodes the body (JSON, text for a string or for an `any` the server does not send as JSON, `[]byte` for binary) |
| `XxxRaw`     | sends; returns a 2xx `*http.Response` with its body unread                                                                 |
| `XxxRequest` | builds the `*http.Request` without sending it                                                                              |

When a 2xx status has no body next to one that has (a 204 beside a 200), or
the result is nullable, the result is a pointer, nil for the bodiless status
or `null`, unless the type is nil-able already or a string. Each method
takes trailing `RequestEditorFn`s, which change the request after it is
built. A status outside 2xx is a `*ResponseError`; its `Model` is the body
decoded into the type the spec gives for that status (the exact code, then
`4XX`/`5XX`, then `default`), nil when the body lacks a property that type
requires. So is a 2xx whose body is of another type than the result (a 202
`Problem` next to a 201 `Order`): `Xxx` returns it as a `*ResponseError`
with that body in `Model`, `XxxRaw` as the response.

A response that streams its items, described by 3.2's `itemSchema` (FastAPI
writes it for a `yield` endpoint, in a 3.1 document), makes `Xxx` return an
`iter.Seq2[T, error]` that yields each item as it arrives: a JSON value per
line for JSON Lines, a `ServerSentEvent[T]` (`Data`, `Event`, `ID`, `Retry`)
per event for `text/event-stream`, `T` being the `contentSchema` of its
`data`, or the text itself. A range stops at the first error. The response
stays open until the range ends, a `break` or `ctx` is cancelled: range the
result, or cancel `ctx`. JSON Lines (`application/jsonl`,
`application/x-ndjson`) and `text/event-stream` are read; another streamed
media type has only `XxxRaw`.

```go
events, err := api.Chat(ctx, petstore.ChatParams{Body: petstore.ChatBody{Prompt: "hi"}})
if err != nil {
	return err
}

for e, err := range events {
	if err != nil {
		return err
	}

	fmt.Println(e.Event, e.Data)
}
```

| `Configuration` | Use                                                                    |
| --------------- | ---------------------------------------------------------------------- |
| `BasePath`      | replaces the spec's first server URL                                   |
| `HTTPClient`    | any `Doer`; nil is `http.DefaultClient`                                |
| `Headers`       | added to every request                                                 |
| `Username`      | with `Password`, HTTP basic auth                                       |
| `APIKey`        | returns the key named `name` for operation `op`                        |
| `AccessToken`   | returns the bearer token for scheme `name`, operation `op`, its scopes |

## Types

| Schema                                    | Go                                                                                                                                                                       |
| ----------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| required property                         | `T`                                                                                                                                                                      |
| optional (or read-only)                   | `*T`; slices, maps and `any` stay as they are: nil is absent, empty is sent                                                                                              |
| required and nullable                     | `*T`, sent as `null` when nil                                                                                                                                            |
| optional and nullable                     | `Opt[T]`: absent, `Null[T]()` or `Some(v)`                                                                                                                               |
| `date` / `date-time`                      | `Date` / `DateTime` (a `time.Time`; one without an offset, a naive FastAPI datetime, is `Naive` and goes back without one)                                               |
| enum                                      | a named type with a constant per value; `any` when the values are of more than one JSON type                                                                             |
| `byte` / `binary` property                | `[]byte` (base64 on the wire) / `string` (the raw text, as pydantic sends `bytes`)                                                                                       |
| inline object                             | a struct named after its owner: `ProfileUser`, `SearchReposResponse`, `CreateCustomerAddress`                                                                            |
| nullable item or result                   | `[]*T`, `map[string]*T`; a result of a nullable model is `*T`                                                                                                            |
| `allOf`                                   | one struct with the members' properties                                                                                                                                  |
| `additionalProperties` next to properties | an `AdditionalProperties` map (numbered on a clash) beside the fields, read and written with them                                                                        |
| `oneOf` / `anyOf`                         | a wrapper over the raw JSON, `AsX` / `FromX`; `AsX` refuses a value without X's required properties; inline, named after its owner as an object is                       |
| ... with a discriminator                  | also `Discriminator()` and `ValueByDiscriminator()`                                                                                                                      |
| ... pinning a property                    | a variant property of one value (`enum: [cat]`, 3.1 `const`) is set by `FromX` and checked by `AsX`; one that tells every variant apart is the discriminator, inline too |
| file in a multipart body                  | `File{Name, Body}`; an `octet-stream` body is `io.Reader`                                                                                                                |
| other multipart field                     | its own part: a list of scalars a part per item (comma-joined with `explode: false`), an object as JSON                                                                  |
| urlencoded form field                     | sent as its `encoding` says: form and exploded by default, `deepObject` key by key (`a[b][0]=x`); a form with a file goes as multipart                                   |

Not covered: server code, XML, server variables, validation, a discriminator
without `oneOf` (the parent stays a plain struct), `additionalProperties`
next to the properties of an inline object (it stays a map) or inherited
through `allOf` (the extra keys are dropped).
