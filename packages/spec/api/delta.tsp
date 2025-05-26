import "@typespec/protobuf";

import "./lib.tsp";

using TypeSpec.Protobuf;

@package({
  name: "delta.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/delta/v1;deltav1",
  },
})
namespace API.Delta;

enum SourceKind {
  SOURCE_KIND_UNSPECIFIED: 0,
  SOURCE_KIND_ORIGIN: 1,
  SOURCE_KIND_MIXED: 2,
  SOURCE_KIND_DELTA: 3,
}
