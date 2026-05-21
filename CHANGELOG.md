# Changelog

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
