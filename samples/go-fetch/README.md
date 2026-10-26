# go-fetch

Sample usage of the `go-fetch` generator.

[Petstore Extended](https://github.com/OAI/OpenAPI-Specification)

The spec is vendored as `petstore-expanded.yaml` and the generator options live
in `config.yaml`, so the sample regenerates byte-for-byte:

```bash
make samples-go-fetch
```

Which is equivalent to:

```bash
go run ./cmd/openapi-generator generate \
    -g go-fetch \
    -i samples/go-fetch/petstore-expanded.yaml \
    -p packageName=petstore \
    -p withGoMod=true \
    -p moduleName=example.com/petstore \
    -o samples/go-fetch/petstore-expanded
```

The output is a module of its own (`withGoMod`), so `go vet ./...` in
`samples/go-fetch/petstore-expanded` compiles it.

Run `openapi-generator config-help go-fetch` for the full list of options.
