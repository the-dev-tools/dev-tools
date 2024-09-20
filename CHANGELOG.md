# @the-dev-tools/protobuf

## 1.22.0

### Minor Changes

- [`ed1dbf4`](https://github.com/the-dev-tools/dev-tools-proto/commit/ed1dbf4a333226628689856f1cd3d64810687638) Thanks [@ElecTwix](https://github.com/ElecTwix)! - add body type to updateExample

## 1.21.0

### Minor Changes

- [`7f0a0f7`](https://github.com/the-dev-tools/dev-tools-proto/commit/7f0a0f7d1be321025ec322582cce3bb100e75fb7) Thanks [@ElecTwix](https://github.com/ElecTwix)! - move body rpc to body.proto and refactor

### Patch Changes

- [`5a57e1b`](https://github.com/the-dev-tools/dev-tools-proto/commit/5a57e1bdeaa77b4ec4e37a3e01a693b2976e2de9) Thanks [@ElecTwix](https://github.com/ElecTwix)! - update BodyFormDataItem data Items to Item
  add Move Call for ApiItem and FolderItem

## 1.20.0

### Minor Changes

- [`76a5120`](https://github.com/the-dev-tools/dev-tools-proto/commit/76a51203891bfde5f1a1bab952b98cd509b42aa4) Thanks [@ElecTwix](https://github.com/ElecTwix)! - add id field to create RPC call

## 1.19.0

### Minor Changes

- [`f044145`](https://github.com/the-dev-tools/dev-tools-proto/commit/f044145a7b95f1a06368f5431fa6c9cae3f104bb) Thanks [@ElecTwix](https://github.com/ElecTwix)! - add example_id to header, query and body

## 1.18.0

### Minor Changes

- [`c9068dd`](https://github.com/the-dev-tools/dev-tools-proto/commit/c9068dd8313c55509a3dad8bd930bed654cbe214) Thanks [@ElecTwix](https://github.com/ElecTwix)! - Refactor the body messages
  Remove create response data

  Added CI for buf linting
  fix couple of linting issues

  added example id to the create request for Header, Query and FormBody
  update lint github action trigger to push on main branch

## 1.17.0

### Minor Changes

- [`6eb1a7f`](https://github.com/the-dev-tools/dev-tools-proto/commit/6eb1a7f9a269c3942d760c4b410c24ff3de96e3f) Thanks [@ElecTwix](https://github.com/ElecTwix)! - move method field from apiCall to apiCallMeta

- [`1247b57`](https://github.com/the-dev-tools/dev-tools-proto/commit/1247b577a773549949a8c0bab359e25f9ffe9eaa) Thanks [@ElecTwix](https://github.com/ElecTwix)! - Removed Created fields favor of Ulid ID field
  Added Headers as own Message
  Added QueryParams as own Message
  Added Body as own Message
  Combine partials create message into original message
  Removed Cookies field

## 1.16.0

### Minor Changes

- [`b58cf68`](https://github.com/the-dev-tools/dev-tools-proto/commit/b58cf68fb646e49ab7331dca4249c68ef96262c9) Thanks [@ElecTwix](https://github.com/ElecTwix)! - added MetaItems to MetaFolder

- [`d1822af`](https://github.com/the-dev-tools/dev-tools-proto/commit/d1822af8fcbc18692e640034043d0cd08ccdc527) Thanks [@ElecTwix](https://github.com/ElecTwix)! - add metaItems to CollectionMeta

## 1.15.0

### Minor Changes

- [`898c0b0`](https://github.com/the-dev-tools/dev-tools-proto/commit/898c0b09522d2726d9291071d2fef6cca8cacb67) Thanks [@ElecTwix](https://github.com/ElecTwix)! - add examples to apiCall

- [`314acb4`](https://github.com/the-dev-tools/dev-tools-proto/commit/314acb470e5aab836ed29c19e6f936544235a56e) Thanks [@ElecTwix](https://github.com/ElecTwix)! - update apiCall to add metaExamples and remove full examples while providing apiCall itself

## 1.14.0

### Minor Changes

- [`1328b90`](https://github.com/the-dev-tools/dev-tools-proto/commit/1328b90f555b7fa29e7bd8a0320429e5d4a7122f) Thanks [@ElecTwix](https://github.com/ElecTwix)! - add go package paths

- [`669df42`](https://github.com/the-dev-tools/dev-tools-proto/commit/669df42ada7c401d23deb6f5c080be1181a0f579) Thanks [@ElecTwix](https://github.com/ElecTwix)! - added timestamps to workspace and collection

- [`cd02ca6`](https://github.com/the-dev-tools/dev-tools-proto/commit/cd02ca6291df9e830a69c198ffadc9d9ed6d0e12) Thanks [@ElecTwix](https://github.com/ElecTwix)! - separate the item services

- [`804a0d7`](https://github.com/the-dev-tools/dev-tools-proto/commit/804a0d72b5f2bb838494acaa28b589a3af12418e) Thanks [@ElecTwix](https://github.com/ElecTwix)! - refactor items

- [`20675d2`](https://github.com/the-dev-tools/dev-tools-proto/commit/20675d2b62507442c34933ca639b3fb576d05ed4) Thanks [@ElecTwix](https://github.com/ElecTwix)! - add enum to workspace/user/role

- [`64d2060`](https://github.com/the-dev-tools/dev-tools-proto/commit/64d2060ea8a527ee980dadb9acd86b6370df72fb) Thanks [@ElecTwix](https://github.com/ElecTwix)! - add user management to workspace

- [`0f7d4f3`](https://github.com/the-dev-tools/dev-tools-proto/commit/0f7d4f36e317898693ab42c1541caaabae331dba) Thanks [@ElecTwix](https://github.com/ElecTwix)! - add parent_id to some collection calls

### Patch Changes

- [`d489f44`](https://github.com/the-dev-tools/dev-tools-proto/commit/d489f44c1bb343194dbb54bf81d7e7fafcc1b61a) Thanks [@ElecTwix](https://github.com/ElecTwix)! - fix order of getCollection

- [`202d1f4`](https://github.com/the-dev-tools/dev-tools-proto/commit/202d1f4bb86588a6c7be2b22fe50b9579a04b91c) Thanks [@ElecTwix](https://github.com/ElecTwix)! - update role type and enum

- [`7115cd4`](https://github.com/the-dev-tools/dev-tools-proto/commit/7115cd412e7a662e4a9493725963186b69fee547) Thanks [@ElecTwix](https://github.com/ElecTwix)! - fix non-imported timestamp

## 1.13.0

### Minor Changes

- [`eea7432`](https://github.com/the-dev-tools/dev-tools-proto/commit/eea74320041eda197507e1fe093649fe78ebea14) Thanks [@ElecTwix](https://github.com/ElecTwix)! - invite add to workspace

## 1.12.3

### Patch Changes

- [`f1e8bad`](https://github.com/the-dev-tools/dev-tools-proto/commit/f1e8bad8a39d74adf1c6f8de892a8b2454281181) Thanks [@Tomaszal](https://github.com/Tomaszal)! - Implement changeset workflow and CI
