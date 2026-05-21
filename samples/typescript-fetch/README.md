# typescript-fetch

Sample usage of the `typescript-fetch` with various OpenAPI specifications.

[Kubernetes API](https://github.com/kubernetes/kubernetes)

```bash
go run ./cmd/openapi-generator generate \
    -g typescript-fetch \
    -p withPackageJson=true \
    -i https://raw.githubusercontent.com/kubernetes/kubernetes/refs/heads/master/api/openapi-spec/swagger.json \
    --skip-validate-spec \
    -o samples/typescript-fetch/kubernetes \
```

[Github API](https://github.com/github/rest-api-description)

```bash
go run ./cmd/openapi-generator generate \
    -g typescript-fetch \
    -p withPackageJson=true \
    -i https://raw.githubusercontent.com/github/rest-api-description/refs/heads/main/descriptions/api.github.com/api.github.com.yaml \
    --skip-validate-spec \
    -o samples/typescript-fetch/github \
```

[Petstore Extended](https://github.com/OAI/OpenAPI-Specification)

```bash
go run ./cmd/openapi-generator generate \
    -g typescript-fetch \
    -p withPackageJson=true \
    -i https://raw.githubusercontent.com/OAI/OpenAPI-Specification/3.1.0/examples/v3.0/petstore-expanded.yaml \
    --skip-validate-spec \
    -o samples/typescript-fetch/petstore-expanded \
```
