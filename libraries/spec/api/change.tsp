import "@typespec/protobuf";

import "../lib";

using TypeSpec.Protobuf;

@package({
  name: "change.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/change/v1;changev1",
  },
})
namespace API.Change;

model Changes {
  @autoField changes?: Change[];
}

enum ChangeKind {
  CHANGE_KIND_UNSPECIFIED: 0,

  /** Merge the new data onto the old data */
  CHANGE_KIND_UPDATE: 1,

  /**
   * Force re-fetch of `service`'s `method` request, where `data` is used as a query filter
   *
   * Useful for non-normalizable data, or large/complex data that may not be needed immediately
   */
  CHANGE_KIND_INVALIDATE: 2,
}

@autoFields
model Change {
  kind: ChangeKind;

  /** Data to change, must be normalizable */
  data: WellKnown.Any;

  /** Fully qualified service name for invalidation */
  service?: string;

  /** Method name of a `service` for invalidation */
  method?: string;
}
