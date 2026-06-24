# Changelog

## [0.1.9](https://github.com/parallelworks/openapi-client-generator/compare/v0.1.8...v0.1.9) (2026-06-24)


### Bug Fixes

* emit required query parameters as positional arguments ([#24](https://github.com/parallelworks/openapi-client-generator/issues/24)) ([629f442](https://github.com/parallelworks/openapi-client-generator/commit/629f4427caf538e125b91b5fea3eafe4e7b590da))

## [0.1.8](https://github.com/parallelworks/openapi-client-generator/compare/v0.1.7...v0.1.8) (2026-06-22)


### Bug Fixes

* retries can duplicate non-idempotent requests on 5xx ([#22](https://github.com/parallelworks/openapi-client-generator/issues/22)) ([907a739](https://github.com/parallelworks/openapi-client-generator/commit/907a73910eca307d38e54cd0fa13566271b7b761))

## [0.1.7](https://github.com/parallelworks/openapi-client-generator/compare/v0.1.6...v0.1.7) (2026-06-22)


### Bug Fixes

* retry idempotent requests on transient network errors ([#20](https://github.com/parallelworks/openapi-client-generator/issues/20)) ([b91321e](https://github.com/parallelworks/openapi-client-generator/commit/b91321ed5f444c14f03f296c37150c158c681df4))

## [0.1.6](https://github.com/parallelworks/openapi-client-generator/compare/v0.1.5...v0.1.6) (2026-06-10)


### Features

* typed discriminated unions for inline oneOf/anyOf schemas ([#18](https://github.com/parallelworks/openapi-client-generator/issues/18)) ([3b276de](https://github.com/parallelworks/openapi-client-generator/commit/3b276decfe67f2753331678602848061af538e45))

## [0.1.5](https://github.com/parallelworks/openapi-client-generator/compare/v0.1.4...v0.1.5) (2026-03-03)


### Bug Fixes

* query params incorrectly generated ([#13](https://github.com/parallelworks/openapi-client-generator/issues/13)) ([bb5c85f](https://github.com/parallelworks/openapi-client-generator/commit/bb5c85f0835b6dbd8b34240b553c0b006a8e338d))

## [0.1.4](https://github.com/parallelworks/openapi-client-generator/compare/v0.1.3...v0.1.4) (2026-03-03)


### Features

* extract OpenAPI docs into Go doc comments ([#11](https://github.com/parallelworks/openapi-client-generator/issues/11)) ([39957df](https://github.com/parallelworks/openapi-client-generator/commit/39957dfb95ac2800935cf4a8ba850a8a3224d716))

## [0.1.3](https://github.com/parallelworks/openapi-client-generator/compare/v0.1.2...v0.1.3) (2026-03-03)


### Bug Fixes

* codegen header is package doc comment ([#9](https://github.com/parallelworks/openapi-client-generator/issues/9)) ([06aadff](https://github.com/parallelworks/openapi-client-generator/commit/06aadffa1a078017240c82a33ad69e55009fa150))

## [0.1.2](https://github.com/parallelworks/openapi-client-generator/compare/v0.1.1...v0.1.2) (2026-03-02)


### Bug Fixes

* text/plain handling ([#7](https://github.com/parallelworks/openapi-client-generator/issues/7)) ([9d19c6e](https://github.com/parallelworks/openapi-client-generator/commit/9d19c6e56b23f4834348b310ec7b562035eca191))

## [0.1.1](https://github.com/parallelworks/openapi-client-generator/compare/v0.1.0...v0.1.1) (2026-03-02)


### Features

* errors.As support for custom errors ([ebc9ef4](https://github.com/parallelworks/openapi-client-generator/commit/ebc9ef4ef4e81476e16934516fa1ed7f1685205a))
* openapi 3.1 generator ([de9ff6d](https://github.com/parallelworks/openapi-client-generator/commit/de9ff6d47c9115a6f9033328f3236d8616ab0968))
* optional params, non-reserved field names have _ ([7d1781b](https://github.com/parallelworks/openapi-client-generator/commit/7d1781b1a80029f2a84c845e0aa067d1d4ce0204))
* use errors.Is instead of helpers ([089ef60](https://github.com/parallelworks/openapi-client-generator/commit/089ef603193314a63c0c2b898e68ba6fbf561ec2))


### Bug Fixes

* header-only params generate unused params variable ([#3](https://github.com/parallelworks/openapi-client-generator/issues/3)) ([cfe8d53](https://github.com/parallelworks/openapi-client-generator/commit/cfe8d53477e809077ca68900bf189e4895a87a1d))
