# End-to-end tests

Generated clients make real calls. Each call passes through the client's
request building, the HTTP wire and its response decoding, and the test
checks what arrived, not only the status code.

```sh
make e2e                              # every generator, both layers
E2E_GENERATOR=go-fetch make e2e       # one generator
E2E_DART=~/dart-sdk/bin/dart make e2e # a Dart other than the one on PATH
make e2e-update                       # rewrites specs/fastapi.json from the backend
```

`go test ./...` skips these tests; `make e2e` sets `E2E=1`. Tools:

| Generator          | Needs                                                                       |
| ------------------ | --------------------------------------------------------------------------- |
| all                | [uv](https://docs.astral.sh/uv/) for the FastAPI backend                    |
| `go-fetch`         | Go                                                                          |
| `typescript-fetch` | `tsc` (`npm i -g typescript`) and Node 22 or later (its `fetch`, `File`)    |
| `dart-fetch`       | Dart 3.13 or later, which the generated pubspec asks for; skipped with less |

A run takes about 7 s with warm caches.

## Layers

| Layer   | Server                                                                                                                | Spec                                                          | Checks                                                                                                       |
| ------- | --------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| Mock    | `internal/e2e`: validates each request against the spec (kin-openapi), records it, answers with the case's `response` | a feature spec, `testdata/specs/feature`                      | the request is valid and arrives as `request` says; the client returns `result` or `error` from the response |
| Backend | `backends/fastapi`: a strict FastAPI app (pydantic rejects extra keys) that echoes what it parsed                     | the one the server serves, compared with `specs/fastapi.json` | the client returns `result` or `error`; the server's validation and echo show what it received               |

The mock layer covers every parameter style and body encoding the feature
specs declare. The backend shows the shapes a framework writes into its
spec: pydantic's separate input and output schemas, OpenAPI 3.1 null unions,
`const`, Decimal as number or string, FastAPI's empty JSON schema next to
another media type, discriminated unions in every position, naive and aware
date-times, required nullables, form models, enums of mixed JSON types,
parameters whose names collide once camelCased, and streamed responses
(JSON Lines and server-sent events from `yield`, 3.2's `itemSchema`).

## Scenarios

`scenarios/<name>.yaml` holds one spec's cases. Every value in it is in
wire terms, with names, JSON and media types as the spec has them. The cases
therefore do not depend on a generator's naming or language.

```yaml
spec: ../../specs/feature/parameters-locations.yaml   # mock layer; or: backend: fastapi
auth: { username: u, password: p, apiKey: k, bearer: t }
cases:
  - name: path, query, header and cookie
    op: getItem                       # operationId
    args: { orgId: acme/eu, itemId: 42, X-Request-Id: r-1, body: {...} }
    request:                          # mock only: what the server must receive
      path: /orgs/acme/eu/items/42    # unescaped
      query: { verbose: "false" }     # a string stands for a one-value list
      header: { X-Request-Id: r-1 }
      cookie: { sessionId: s-1 }
      body: {...}                     # JSON; a form as lists; multipart parts
    response: { status: 200, json: { id: "42" } }   # mock only; or text, base64, header
    result: { id: "42" }              # the client's result as wire JSON; null is null
    error: { status: 404, model: {...}, body: "..." }
    save: { token: access_token }     # ${token} in later args and auth
    skip: { go-fetch: why it fails }  # a known gap; fails once it passes
```

The body argument is `body`; a form's fields are arguments of their own.
A file is `{ $file: { name, text | base64 } }` and a raw body is
`{ $bytes: { base64 } }`.

**Matching.** An expected object lists the keys it checks, and other keys
may be present. An array must hold exactly the listed items. `$any` matches
any value, `$absent` requires a missing key, and numbers compare exactly
(int64 and beyond). In a `request`, a string also matches a one-value list,
and header names match in any case.

**What the mock records.**
- The path both unescaped (`path`) and as sent (`rawPath`).
- A body only when one was sent: JSON as JSON, a form as lists of values, a
  multipart field as a list of parts, text as a string, anything else as
  `{ $base64 }`. A file part arrives as
  `{ filename, contentType, text | base64 }` (text when its bytes are UTF-8),
  a JSON part as `{ json: ... }`.

Every request must pass the spec's validation, and so must the scenario's
`response`, unless it sets `offSpec: true` (an undeclared status on purpose).
A backend scenario has no `request` or `response`: the server answers and
validates.

**Portable values.** A case states what every generator should do. A value
that a language cannot represent the same way belongs in a case of its own,
which that generator's runner skips: an int64 above 2^53 in JavaScript, a
time with microseconds in a JavaScript `Date`, an error body a client does
not decode.

## Runners

A runner turns the scenarios into calls of one generator's client and uses
the client the way its users do. `internal/e2e` generates every suite's
client, builds one program that holds them all and runs it, then compares
what the program prints with the scenarios.

| Runner         | Client                                                                                                                                                                 |
| -------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `runners/go`   | go-fetch: finds methods, `Params` fields and results by reflection; standard library only                                                                              |
| `runners/ts`   | typescript-fetch, compiled to CommonJS with `tsc`: reads the `<Op>Request` interfaces and method signatures from the sources, converts with `<Model>FromJSON`/`ToJSON` |
| `runners/dart` | dart-fetch, on the VM with `dart:mirrors`: named parameters, `fromJson`/`toJson`; a client that does not compile fails only its own suite                              |

Runners run with `TZ=Europe/Bratislava`, so a client that reads or writes
local time shows it.

typescript-fetch sends an OAuth2 token as the whole `Authorization` header,
so its runner returns `Bearer <token>` for an OAuth2 scheme (`schemes` in the
input). Its error has no decoded model: the runner reports the error body
parsed as JSON. A case one generator cannot pass has
`skip: { <generator>: why }`. A spec whose client does not compile at all
has the same `skip` at the top of its file.

A runner for another generator follows the same protocol:

1. **Input** is a JSON file given as the first argument:
   `{ suites: [{ name, base, mock, auth, schemes, cases: [{ op, params, args, auth, save }] }] }`.
   - `params` lists the names the operation's args may use (parameters and
     form fields); any other arg is `body`.
   - A case's `auth` fields replace those before it for the rest of the
     suite.
2. **Before each case** of a `mock` suite, the runner sends
   `POST {base}/__e2e/case/{index}` so that the mock knows which case the
   next request belongs to.
3. **Arguments.** `${name}` in `args` and `auth` is a value an earlier
   case saved:
   - `save: { name: path }` takes a dotted path into the result
     (`items.0.id`).
   - A string is saved as it is, anything else as its JSON.
   - A name never saved fails the case.
4. **Output** is one JSON line per case, `{ suite, case, result | error | fail }`:
   - `result` is the client's result as wire JSON; a binary result is a
     base64 string, a streamed one the list of its items (an event as
     `{ data, event, id, retry }`), and a call without one is `null`.
   - `error` is `{ status, model, body }` from the client's error for a
     status (`model` is the decoded error body, or absent).
   - `fail` says why the call did not happen or failed otherwise (an
     unknown arg, a transport error, a panic).

   A case the runner cannot handle fails that case only, not the run.
