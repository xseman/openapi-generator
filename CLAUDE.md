# openapi-generator

A Go port of the Java OpenAPI Generator's CLI: one static binary that turns an
OpenAPI 3.x (or Swagger 2.0) spec into a typed client through the upstream
Mustache templates. Read `README.md` for what it does; the map below says
where things live.

## Commands

```sh
make build              # bin/openapi-generator, version from the release manifest
make test               # go vet + go test -race ./...
make lint               # golangci-lint, config in .golangci.yml — must be clean
make fmt                # gofumpt -w .
make cover              # coverage.out + per-function summary
go test ./internal/parser -run TestName
```

`make test` and `make lint` are what CI runs (`.github/workflows/quality.yml`).

Go 1.24, stdlib + kin-openapi (spec loading), cbroglie/mustache, cobra,
yaml.v3, x/text. No cgo.

## Layout

| Path                            | Contents                                                                    |
| ------------------------------- | --------------------------------------------------------------------------- |
| `cmd/openapi-generator`         | cobra CLI: `generate`, `validate`, `list`, `config-help`, `version`, `update` |
| `internal/parser`               | kin-openapi document → `codegen` structs: models, operations, parameters    |
| `internal/codegen`              | the `Codegen*` structs templates render; fields mirror the Java originals   |
| `internal/gen`                  | the pipeline: load spec → parse → render models, apis, supporting files     |
| `internal/generator`            | `CodegenConfig` and the language configs, `typescript/` and `dart/`         |
| `internal/template`             | the Mustache engine: partials, lambdas, struct-to-map via json              |
| `internal/config`               | generator options and their config-file keys                                |
| `internal/update`               | release check, verified download, self-update                               |
| `templates/`                    | the `.mustache` files, embedded by `templates/embed.go`                     |
| `samples/`                      | configs and generated output kept as examples                               |

## Conventions

- Generated client code is never edited by hand: fix the template under
  `templates/<generator>/` or the logic in `internal/generator/` or
  `internal/parser/`, then regenerate. Delete `generated/` first so stale
  files do not survive.
- `codegen`, `config` and `generator` mirror the Java openapi-generator's
  names one for one (`OperationIdCamelCase`, `IsUuid`, `ApiPackage`), so a
  template author can read the upstream docs. Keep them, even where Go would
  spell them differently; `.golangci.yml` excludes those packages from
  revive's naming rules for this reason and no other.
- Templates see a struct through `json.Marshal`, so the `json` tag is the
  name a template uses. Renaming a Go field is free; renaming a tag breaks
  every template that reads it.
- The output must stay stable across runs: iterate maps in sorted order
  wherever the result reaches a file.
- Tests: table tests over small specs in the package that owns the logic;
  `internal/parser` has the most. A parser for a spec fragment gets a case for
  the fragment, not a full petstore.
- `make lint` must stay clean. Never silence a linter to get there: fix the
  code, or write `//nolint:<linter> // <reason>` — `nolintlint` rejects a
  directive without both. A deliberately dropped error reads `_ = f()`.
- Doc comments start with the identifier and end with a period. Exported API
  gets one; so does anything whose behaviour the name does not give away.
- `// ponytail:` marks a deliberate shortcut left for later, with its ceiling
  and the upgrade path. Keep them true.
- Format with `make fmt` (gofumpt). Tabs in Go and the Makefile; Markdown
  tables aligned with spaces.
- Code reads in paragraphs: a step and its check sit together, a blank line
  separates it from the next step, from a `case` longer than two lines, and
  from a closing `return`. A block (`if`, `for`, `switch`) cuddles only with
  the one line it uses. `wsl_v5` and `nlreturn` enforce it;
  `golangci-lint run --fix` inserts the lines.
- Commit only when asked, as a conventional commit: release-please builds the
  changelog from the types (`feat`, `fix`, `refactor`, `docs`, `ci`, …).

## Gotchas

- The version is `internal/update.Version`, linked in with `-X` by the
  Makefile and `release.yml` without its `v`; a plain `go build` reports
  `dev` and `update` then installs the latest release.
- Release asset names are API: `openapi-generator-<os>-<arch>[.exe]` plus
  `CHECKSUMS.txt`. `install.sh` and `update` refuse anything the checksum file
  does not cover, so add platforms, never rename.
- `--fix` from golangci-lint can leave a file needing an import it did not
  add (`errors` after perfsprint); `goimports -w` repairs that.
