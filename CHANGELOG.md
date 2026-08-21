# Changelog

## [0.2.9](https://github.com/parallelworks/openapi-client-generator/compare/v0.2.8...v0.2.9) (2026-08-21)


### Features

* **analyzer:** a const says what type a property has ([#118](https://github.com/parallelworks/openapi-client-generator/issues/118)) ([82d2af7](https://github.com/parallelworks/openapi-client-generator/commit/82d2af7e37fa93bb8386c10f0e1d89f428ccbee4))
* **analyzer:** a union parameter names a type instead of resting at any ([#88](https://github.com/parallelworks/openapi-client-generator/issues/88)) ([15fdb23](https://github.com/parallelworks/openapi-client-generator/commit/15fdb238bdd9f99e560674ef64ad88b4e9fc6ecf))
* **analyzer:** allOf is honored wherever it appears, not only on named schemas ([#117](https://github.com/parallelworks/openapi-client-generator/issues/117)) ([41e0737](https://github.com/parallelworks/openapi-client-generator/commit/41e073725beb9b6741d91ed4e2709e34611e0c70))
* **analyzer:** an object written inline keeps the properties it declares ([#80](https://github.com/parallelworks/openapi-client-generator/issues/80)) ([01c0eb5](https://github.com/parallelworks/openapi-client-generator/commit/01c0eb5dfb11885c930b7238379fee6b4919dc84))
* **analyzer:** patternProperties types its map, and what stays unsupported says so ([#84](https://github.com/parallelworks/openapi-client-generator/issues/84)) ([f57cff2](https://github.com/parallelworks/openapi-client-generator/commit/f57cff2a42eecf6526baa847b8dcd619f71f888a))
* **generator:** a text/event-stream response is read as it arrives ([#124](https://github.com/parallelworks/openapi-client-generator/issues/124)) ([84c7655](https://github.com/parallelworks/openapi-client-generator/commit/84c765524bed257d4e20e580d681456336ed3613))
* **generator:** an error body names its message, extension or not ([#89](https://github.com/parallelworks/openapi-client-generator/issues/89)) ([856692b](https://github.com/parallelworks/openapi-client-generator/commit/856692b8d1523615d53879e5620b515fffe27524))
* **generator:** offset and page pagination get the iterator they were detected for ([#93](https://github.com/parallelworks/openapi-client-generator/issues/93)) ([310d0f0](https://github.com/parallelworks/openapi-client-generator/commit/310d0f094c2f61f3466f356d97108c5678000e0c))
* **generator:** webhooks and callbacks get payload types and a dispatcher ([#86](https://github.com/parallelworks/openapi-client-generator/issues/86)) ([6254c7e](https://github.com/parallelworks/openapi-client-generator/commit/6254c7e4b6eb45d5b3afe40301b7bd234054f3c9))
* **pagination:** an endpoint returning the array itself gets an iterator ([#113](https://github.com/parallelworks/openapi-client-generator/issues/113)) ([7b85a94](https://github.com/parallelworks/openapi-client-generator/commit/7b85a94dbd80555804e7852537ac94209a6241a4))


### Bug Fixes

* **analyzer:** a multipart body written inline sent its file as base64 text ([#91](https://github.com/parallelworks/openapi-client-generator/issues/91)) ([1c185c7](https://github.com/parallelworks/openapi-client-generator/commit/1c185c71b012b420a13e606ff93b7c5dd7a44722))
* **analyzer:** a parameter serialized with content went out style-encoded ([#82](https://github.com/parallelworks/openapi-client-generator/issues/82)) ([9925271](https://github.com/parallelworks/openapi-client-generator/commit/9925271b571de9ca49c60959b91e1d0a3406b037))
* **analyzer:** allOf entries that redeclare a property lost it on the wire ([#121](https://github.com/parallelworks/openapi-client-generator/issues/121)) ([1499cc8](https://github.com/parallelworks/openapi-client-generator/commit/1499cc82cfbce5658b3bc83f696ca56901c8c189))
* **analyzer:** two properties that sanitize to one Go name broke the build ([#97](https://github.com/parallelworks/openapi-client-generator/issues/97)) ([58ef8ec](https://github.com/parallelworks/openapi-client-generator/commit/58ef8ecd0cb507dbccee06de6d3578729474bd42))
* **generator:** a binary or text response body was run through json.Unmarshal ([#85](https://github.com/parallelworks/openapi-client-generator/issues/85)) ([01305e6](https://github.com/parallelworks/openapi-client-generator/commit/01305e6cd5103e8f02c75b2d9f39af6261ca4bdd))
* **generator:** allowReserved escaped the characters it says to pass through ([#83](https://github.com/parallelworks/openapi-client-generator/issues/83)) ([3f39e28](https://github.com/parallelworks/openapi-client-generator/commit/3f39e284bbf6c0e0d66aff473f2d30b147f58b38))
* **generator:** an operation declaring security: [] still got the credential ([#107](https://github.com/parallelworks/openapi-client-generator/issues/107)) ([afae0d8](https://github.com/parallelworks/openapi-client-generator/commit/afae0d8a7260f170aac14e5512fd8a6e0294d806))
* **generator:** an optional body left out was sent as the JSON literal null ([#103](https://github.com/parallelworks/openapi-client-generator/issues/103)) ([da687a9](https://github.com/parallelworks/openapi-client-generator/commit/da687a9563b6f2ad848850d2d5f1edf966ae648f))
* **generator:** operations, header fields, and webhooks redeclared derived names ([#99](https://github.com/parallelworks/openapi-client-generator/issues/99)) ([ce6bee3](https://github.com/parallelworks/openapi-client-generator/commit/ce6bee304637bf26e2a408f4541e4580b4b0ff65))
* **generator:** Retry-After was read in one form and obeyed without limit ([#95](https://github.com/parallelworks/openapi-client-generator/issues/95)) ([f82367d](https://github.com/parallelworks/openapi-client-generator/commit/f82367dcf3c1cdb063b23f807341751243de6f3e))
* **pagination:** a page or offset that is not a number broke the build ([#115](https://github.com/parallelworks/openapi-client-generator/issues/115)) ([1ca7c29](https://github.com/parallelworks/openapi-client-generator/commit/1ca7c29f1ecd0cb9595f5c3440673ec1d6274e08))
* **pagination:** a page size named perPage was not recognized ([#109](https://github.com/parallelworks/openapi-client-generator/issues/109)) ([e43dd92](https://github.com/parallelworks/openapi-client-generator/commit/e43dd922ac429b4587c55546ea6495e606da0895))
* **pagination:** skip was not recognized as an offset ([#111](https://github.com/parallelworks/openapi-client-generator/issues/111)) ([6042c3b](https://github.com/parallelworks/openapi-client-generator/commit/6042c3ba43aa60d8544d23e2898d9322be4f3890))
* **parser:** a relative $ref resolved against the working directory ([#106](https://github.com/parallelworks/openapi-client-generator/issues/106)) ([518c879](https://github.com/parallelworks/openapi-client-generator/commit/518c879d8a8eaea20c32b0e319d881256985f0be))
* **templates:** a schema named DefaultBaseURL redeclared the client's own ([#101](https://github.com/parallelworks/openapi-client-generator/issues/101)) ([3b45efa](https://github.com/parallelworks/openapi-client-generator/commit/3b45efa1895b55f3b829a9f55690a39dc62453f8))

## [0.2.8](https://github.com/parallelworks/openapi-client-generator/compare/v0.2.7...v0.2.8) (2026-08-20)


### Features

* **generator:** response status and headers reach the caller ([#73](https://github.com/parallelworks/openapi-client-generator/issues/73)) ([13e598b](https://github.com/parallelworks/openapi-client-generator/commit/13e598b21d755f8730fc0e40ee095397e438fbb2))
* **generator:** the client knows the server URL the spec declares ([#72](https://github.com/parallelworks/openapi-client-generator/issues/72)) ([dee1161](https://github.com/parallelworks/openapi-client-generator/commit/dee1161de57e21525db9e7331381a559f35b3383))
* **generator:** unions expose a base even when the spec inlines the shared properties ([#59](https://github.com/parallelworks/openapi-client-generator/issues/59)) ([1264826](https://github.com/parallelworks/openapi-client-generator/commit/1264826523a8f19b83582b42206c055e511566bd))


### Bug Fixes

* **analyzer:** a security scheme the generator cannot use costs the whole client ([#64](https://github.com/parallelworks/openapi-client-generator/issues/64)) ([15a9c94](https://github.com/parallelworks/openapi-client-generator/commit/15a9c9471e443933b6886a8f8ac42e1c5108088a))
* **generator:** error body with no type name emits code that is not Go ([#63](https://github.com/parallelworks/openapi-client-generator/issues/63)) ([11bcd44](https://github.com/parallelworks/openapi-client-generator/commit/11bcd4462953dbb4258347672db203dd0bc96ea7))
* **pagination:** iterator paged whichever array the spec declared first ([#66](https://github.com/parallelworks/openapi-client-generator/issues/66)) ([fcc1e33](https://github.com/parallelworks/openapi-client-generator/commit/fcc1e338a83a8ffd64ed3e29ae45b9a594f2399c))

## [0.2.7](https://github.com/parallelworks/openapi-client-generator/compare/v0.2.6...v0.2.7) (2026-08-19)


### Features

* **generator:** unions expose the base every variant composes ([#57](https://github.com/parallelworks/openapi-client-generator/issues/57)) ([d9de8a1](https://github.com/parallelworks/openapi-client-generator/commit/d9de8a10691ab8b7d6342ef5587083ec5cd92bca))

## [0.2.6](https://github.com/parallelworks/openapi-client-generator/compare/v0.2.5...v0.2.6) (2026-08-05)


### Bug Fixes

* embedded schema's additionalProperties swallows the outer schema's declared fields ([#53](https://github.com/parallelworks/openapi-client-generator/issues/53)) ([034c2d1](https://github.com/parallelworks/openapi-client-generator/commit/034c2d19609e783a7af9420cb8061c3766c01538))
* valid specs generate unusable params, bodies, and enum constants ([#49](https://github.com/parallelworks/openapi-client-generator/issues/49)) ([d4c29cc](https://github.com/parallelworks/openapi-client-generator/commit/d4c29cc34122581446197d7612ef438fd4709c80))

## [0.2.5](https://github.com/parallelworks/openapi-client-generator/compare/v0.2.4...v0.2.5) (2026-08-05)


### Bug Fixes

* valid specs lose data, duplicate keys, or fail to compile ([#47](https://github.com/parallelworks/openapi-client-generator/issues/47)) ([99ef133](https://github.com/parallelworks/openapi-client-generator/commit/99ef133d8c9e0f5eea6b1a11a37cce6c42e79f53))

## [0.2.4](https://github.com/parallelworks/openapi-client-generator/compare/v0.2.3...v0.2.4) (2026-08-04)


### Bug Fixes

* additionalProperties are dropped on decode and sent as a "-" key ([#40](https://github.com/parallelworks/openapi-client-generator/issues/40)) ([a4591a5](https://github.com/parallelworks/openapi-client-generator/commit/a4591a569386db2859ab4ffc8d2e904197d29626))
* an unknown discriminator value no longer fails the whole decode ([#41](https://github.com/parallelworks/openapi-client-generator/issues/41)) ([ff8808c](https://github.com/parallelworks/openapi-client-generator/commit/ff8808c138745f1c61ca5c99bf1c66c3d9a4fc95))

## [0.2.3](https://github.com/parallelworks/openapi-client-generator/compare/v0.2.2...v0.2.3) (2026-08-04)


### Bug Fixes

* unions used as a request or response body generate typed instead of any ([#36](https://github.com/parallelworks/openapi-client-generator/issues/36)) ([a40e2ca](https://github.com/parallelworks/openapi-client-generator/commit/a40e2ca176ba133b2561aaa0a5cc7313aa4bacc0))

## [0.2.2](https://github.com/parallelworks/openapi-client-generator/compare/v0.2.1...v0.2.2) (2026-07-19)


### Features

* configurable User-Agent for generated clients ([#34](https://github.com/parallelworks/openapi-client-generator/issues/34)) ([7185ffc](https://github.com/parallelworks/openapi-client-generator/commit/7185ffc75bc4772d5d63cc79b404e88dda31a5ea))

## [0.2.1](https://github.com/parallelworks/openapi-client-generator/compare/v0.2.0...v0.2.1) (2026-07-18)


### Features

* parsed error responses render the x-ms-primary-error-message field ([#31](https://github.com/parallelworks/openapi-client-generator/issues/31)) ([7388473](https://github.com/parallelworks/openapi-client-generator/commit/7388473829dccc1edd6d0d550b100c88356a090a))

## [0.2.0](https://github.com/parallelworks/openapi-client-generator/compare/v0.1.10...v0.2.0) (2026-07-01)


### ⚠ BREAKING CHANGES

* pass required query, header, and cookie params via a named Params struct ([#26](https://github.com/parallelworks/openapi-client-generator/issues/26))

### Features

* pass required query, header, and cookie params via a named Params struct ([#26](https://github.com/parallelworks/openapi-client-generator/issues/26)) ([c413dcb](https://github.com/parallelworks/openapi-client-generator/commit/c413dcb199dc2e55ecf4bfcb8259e044c195899f))

## [0.1.10](https://github.com/parallelworks/openapi-client-generator/compare/v0.1.9...v0.1.10) (2026-07-01)


### Bug Fixes

* stop duplicating the status code in generated API error strings ([#27](https://github.com/parallelworks/openapi-client-generator/issues/27)) ([f534618](https://github.com/parallelworks/openapi-client-generator/commit/f5346182d6ef271bc068d2b9bdf4e45cc2fcf2b2))

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
