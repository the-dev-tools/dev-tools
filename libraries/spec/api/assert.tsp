import "@typespec/protobuf";
import "@typespec/rest";
import "../lib";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "assert.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/assert/v1;assertv1",
  },
})
namespace API.Assert;

enum AssertKind {
  ASSERT_KIND_UNSPECIFIED: 0,
  ASSERT_KIND_EQUAL: 1,
  ASSERT_KIND_NOT_EQUAL: 2,
  ASSERT_KIND_CONTAINS: 3,
  ASSERT_KIND_NOT_CONTAINS: 4,
  ASSERT_KIND_GREATER: 5,
  ASSERT_KIND_LESS: 6,
  ASSERT_KIND_GREATER_OR_EQUAL: 7,
  ASSERT_KIND_LESS_OR_EQUAL: 8,
}

enum PathKind {
  PATH_KIND_UNSPECIFIED: 0,
  PATH_KIND_INDEX: 1,
  PATH_KIND_INDEX_ANY: 2,
}

@autoFields
model PathKey {
  kind: PathKind;
  key?: string;
  index?: int32;
}

