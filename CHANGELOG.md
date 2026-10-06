# Changelog

## [0.3.0](https://github.com/xseman/openapi-generator/compare/v0.2.1...v0.3.0) (2026-10-06)


### Features

* **typescript-fetch:** name downloaded files from Content-Disposition ([673753c](https://github.com/xseman/openapi-generator/commit/673753c01baa42f6fbfeedd40595c96792c88938))


### Bug Fixes

* **dart-fetch:** end enum values with a semicolon ([3d3574e](https://github.com/xseman/openapi-generator/commit/3d3574e2f226c3538e6781319f163c0320c11836))
* **parser:** drop null from enum members, name negative numbers apart ([a8c41cd](https://github.com/xseman/openapi-generator/commit/a8c41cdc14e393005d2974cd4d7f44f68f204612))
* **parser:** keep an untyped all-number enum numeric ([20ab9d9](https://github.com/xseman/openapi-generator/commit/20ab9d9eae36a963da1dfd1d30b79c885d64786e))
* **parser:** quote the values of untyped enums, escape backslashes ([849e77e](https://github.com/xseman/openapi-generator/commit/849e77e440ed89d38f05084cf3446ddab9b599e2))
* **parser:** suffix enum members whose values name alike ([1149242](https://github.com/xseman/openapi-generator/commit/114924227e9fe4fdeccec3a4863494af79718d61))
* **typescript-fetch:** enforce not enum and single-value tags in type guards ([0a1f5cb](https://github.com/xseman/openapi-generator/commit/0a1f5cb016c22bb7ee46d78250bcabbe84e4331e))
* **typescript-fetch:** explode object query parameters ([ff63bf3](https://github.com/xseman/openapi-generator/commit/ff63bf3f78e7ac1e5f4425c9a40b5e170264619b))
* **typescript-fetch:** keep null in a oneOf with one nullable member ([35ad535](https://github.com/xseman/openapi-generator/commit/35ad53516b6284619f180ed0f652da23c4d532ca))
* **typescript-fetch:** leave read-only properties out of request bodies ([5b29e37](https://github.com/xseman/openapi-generator/commit/5b29e379ced56e5d72b4c341e686aeddb7ff6c9b))
* **typescript-fetch:** map each element of a map-of-arrays response ([2604b17](https://github.com/xseman/openapi-generator/commit/2604b178494629908bad3451c070d4441fe04436))
* **typescript-fetch:** parse and serialize dates through runtime helpers ([c79a037](https://github.com/xseman/openapi-generator/commit/c79a037b453839d1f98e96b69602db696018ebd4))
* **typescript-fetch:** render the members of top-level enum schemas ([f935f62](https://github.com/xseman/openapi-generator/commit/f935f62ea01078dec901c52fb4b98fc543d45d9a))
* **typescript-fetch:** require inherited properties a model lists as required ([0c62eac](https://github.com/xseman/openapi-generator/commit/0c62eaca19815b84c10adf8bed5d60aceed73eef))
* **typescript-fetch:** send a multipart array of models as one JSON part ([532dd4a](https://github.com/xseman/openapi-generator/commit/532dd4af288f3f133bbb960e33eee5d44591a674))
* **typescript-fetch:** serialize JSON header parameter content ([b56fe7a](https://github.com/xseman/openapi-generator/commit/b56fe7aa3e343c91146429666c4ffbef332f9169))


### Build System

* add samples-dart-fetch target the dart-fetch README refers to ([612cdc1](https://github.com/xseman/openapi-generator/commit/612cdc183971777ba1918d0139c165ed5d4c7ff1))


### Maintenance

* ignore the root CLI build, end samples gitignore with a newline ([d240411](https://github.com/xseman/openapi-generator/commit/d2404110e524f55bce8bfbc4a15f1b85d6101a07))
* **parser:** fold the oneOf null count into the member loop ([50097c2](https://github.com/xseman/openapi-generator/commit/50097c258a43a85c9bcb7e0868fc8cdc42e234d2))
* **typescript-fetch:** drop the dead isNull branch of enum values ([7f4dbae](https://github.com/xseman/openapi-generator/commit/7f4dbaeec35ff75cd773a12984ea9067e46d3941))


### Testing

* add an enum edge-value feature spec ([e95e49a](https://github.com/xseman/openapi-generator/commit/e95e49a508e1cc7e0c7ab15b01aa1104aa8702f9))
* add feature specs and make regress ([7cca37c](https://github.com/xseman/openapi-generator/commit/7cca37cfce0fdf2d6dfbb95db121e8ffcb1654f6))
* flip each generator option on its own in make regress ([b4c7572](https://github.com/xseman/openapi-generator/commit/b4c7572895a863838a30f8b3293fb0231a3aa474))
* **typescript-fetch:** shorten container serialization tests ([300e307](https://github.com/xseman/openapi-generator/commit/300e30799190b76a15bab596d521b90a53a54ccd))

## [0.2.1](https://github.com/xseman/openapi-generator/compare/v0.2.0...v0.2.1) (2026-09-19)


### Maintenance

* self-update, one-runner release ([#7](https://github.com/xseman/openapi-generator/issues/7)) ([e21224d](https://github.com/xseman/openapi-generator/commit/e21224d1226065d3fdafd3f636c339d279064cca))

## [0.2.0](https://github.com/xseman/openapi-generator/compare/v0.1.1...v0.2.0) (2026-09-09)


### Features

* **cli:** add validate command ([96352f1](https://github.com/xseman/openapi-generator/commit/96352f11d71e8b2e7eaa7fe560d48b73822836e5))
* **dart-fetch:** add dart-fetch generator ([915026e](https://github.com/xseman/openapi-generator/commit/915026eecba716acd92de62943008b679cec96c6))


### Bug Fixes

* **typescript-fetch:** align identifier sanitization with upstream ([3a73bef](https://github.com/xseman/openapi-generator/commit/3a73bef3b72ceb049ce1bb040c66a2da7f31e0c8))
* **typescript-fetch:** array-enum naming and ES2022 lib requirement ([0b9ca0d](https://github.com/xseman/openapi-generator/commit/0b9ca0d8793c153dda6c8479cd0f094d8464df89))
* **typescript-fetch:** date, map & composition codegen + stable output ([a21985c](https://github.com/xseman/openapi-generator/commit/a21985c75542d6ee67cc6bc45bb86b12cc9f04f0))
* **typescript-fetch:** expand form params, fix enum names, reserved-keyword escaping, and AbortError handling ([92709a3](https://github.com/xseman/openapi-generator/commit/92709a3d89d9989381c26878fc550fb39a268da8))
* **typescript-fetch:** fix codegen bugs in parser and templates ([f42207f](https://github.com/xseman/openapi-generator/commit/f42207fa0f694052bd2a3e209fb6008afcc9cefc))
* **typescript-fetch:** normalize generated whitespace and JSDoc formatting ([585c9bc](https://github.com/xseman/openapi-generator/commit/585c9bce46ad16d5efee6e4e4b86ecc38fb22cb8))
* **typescript-fetch:** nullable, oneOf and import fixes in templates ([a3d7e17](https://github.com/xseman/openapi-generator/commit/a3d7e1714efd467f3d6a2fc4f7c8bcbf01c91cc6))
* **typescript-fetch:** param enum quoting and inline map oneOf members ([b07525c](https://github.com/xseman/openapi-generator/commit/b07525cd7b0032e01649421a9fc185b1dbbe7590))
* **typescript-fetch:** serialize date and date-time fields and params ([17b9d82](https://github.com/xseman/openapi-generator/commit/17b9d826e4f7f67699c9f391ddaa403d71f539eb))


### Documentation

* **dart-fetch:** add samples README and link it from the root README ([6d69648](https://github.com/xseman/openapi-generator/commit/6d696484e998549792d14d26367330c04bdb21a4))
* document dart-fetch generator ([342b7a9](https://github.com/xseman/openapi-generator/commit/342b7a9854859597604fc8bfda10d13b665c178a))


### Maintenance

* **cli:** move generation pipeline into internal/gen ([3a7166d](https://github.com/xseman/openapi-generator/commit/3a7166de5df0b55bfe7f933e4225b1bed389edcf))
* **parser:** split openapi.go into per-concern files ([cfa0eab](https://github.com/xseman/openapi-generator/commit/cfa0eab5ffbf5e06fca834b29f8a9b583502b754))

## [0.1.1](https://github.com/xseman/openapi-generator/compare/v0.1.0...v0.1.1) (2026-05-21)


### Bug Fixes

* **build:** make golangci-lint latest release pass ([8c407c4](https://github.com/xseman/openapi-generator/commit/8c407c40e28b3f1bfae151ca8d7afcc97e051716))
* **typescript-fetch:** fix instanceOf guards and container-item serialization ([72ee877](https://github.com/xseman/openapi-generator/commit/72ee8775fe3ae76da8434ede51641ebf63c30aaa))
* **typescript-fetch:** fix multipart requests with container-type params ([04321ef](https://github.com/xseman/openapi-generator/commit/04321eff43e6622f2eb6f28bdf4419c7f12502b1))
* **typescript-fetch:** fix TypeScript 'every on never' error in oneOf models ([f043925](https://github.com/xseman/openapi-generator/commit/f0439255555bc8645f0ed19448ef75c75943f17d))
* **typescript-fetch:** restore error prototype chain in error constructors ([5d0c569](https://github.com/xseman/openapi-generator/commit/5d0c56930fb51817d54de2cc02b78e940b7de7d4))
* **typescript-fetch:** use datatypeWithEnum for interface property types ([9562181](https://github.com/xseman/openapi-generator/commit/9562181018cca02ff46a13ac3ff70b940dd57120))


### Documentation

* add convention to not include Copilot co-author trailer in commits ([16b0a5d](https://github.com/xseman/openapi-generator/commit/16b0a5df3cbca95138cb4aa1b2b1c9da5036d821))
* update README with installation instructions and generator docs ([90ee55d](https://github.com/xseman/openapi-generator/commit/90ee55dbdf844c5eacfccb65757c54faed25baa2))


### Automation

* add windows arm64 & release VERSION injection ([cd633d1](https://github.com/xseman/openapi-generator/commit/cd633d15cd8cdaabe78f85d0208b3e5fecf9b856))
* **release:** update workflow ([19de66b](https://github.com/xseman/openapi-generator/commit/19de66b22c612ff7b7befef5f1a151d80d17a875))


### Maintenance

* **samples:** add typescript-fetch generated samples ([c00f67c](https://github.com/xseman/openapi-generator/commit/c00f67c35d4afeb919945edc9a74418a4b974ab4))
* **typescript-fetch:** bump TypeScript devDependency to 6.0.3 ([d651de4](https://github.com/xseman/openapi-generator/commit/d651de4836212022509595dbc2f66a84f93299e1))

## [0.1.0](https://github.com/xseman/openapi-generator/compare/v0.1.0...v0.1.0) (2026-02-04)


### Features

* initial implementation with typescript-fetch client template ([177ac96](https://github.com/xseman/openapi-generator/commit/177ac96cdd9e3168594ad1beb9d6aca488fabad8))


### Bug Fixes

* embed templates using go:embed ([#4](https://github.com/xseman/openapi-generator/issues/4)) ([1265378](https://github.com/xseman/openapi-generator/commit/12653785361f23b134add782d2fad30291da9f8d))
* improve parser & generator type safety and conflict handling ([eb91415](https://github.com/xseman/openapi-generator/commit/eb91415581825530db9c4478ceb06780d2d67083))


### Automation

* add quality & coverage checks and release artifacts ([6dadba9](https://github.com/xseman/openapi-generator/commit/6dadba945aad466419d33834ac96b79bd680640c))
* update token for release-please action ([4fb9343](https://github.com/xseman/openapi-generator/commit/4fb9343c7f9e23f745a0ce46141b6faf3e0f154f))
