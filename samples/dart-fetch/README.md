# dart-fetch

Sample usage of the `dart-fetch` with various OpenAPI specifications.

[Petstore Extended](https://github.com/OAI/OpenAPI-Specification)

The spec is vendored as `petstore-expanded.yaml` and the generator options live
in `config.yaml`, so the sample regenerates byte-for-byte:

```bash
make samples-dart-fetch
```

Which is equivalent to:

```bash
go run ./cmd/openapi-generator generate \
    -c samples/dart-fetch/config.yaml
```

Or with the same options spelled out on the command line:

```bash
go run ./cmd/openapi-generator generate \
    -g dart-fetch \
    -i samples/dart-fetch/petstore-expanded.yaml \
    -p pubName=petstore_api \
    -p pubVersion=1.0.0 \
    -p 'pubDescription=Petstore API client (dart-fetch sample)' \
    -p 'sdkConstraint=>=3.13.0-0 <4.0.0' \
    -p useDartIoSender=true \
    -o samples/dart-fetch/petstore-expanded
```

Run `openapi-generator config-help dart-fetch` for the full list of options.
