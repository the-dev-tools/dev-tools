import "@typespec/protobuf";

import "./lib.tsp";

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

model Source {
  kind: SourceKind;
  $type: Reflection.Model;
}

enum SourceKind {
  REQUEST,
  RESPONSE,
  MERGE,
}

@withoutOmittedProperties("parent")
model AutoListChange {
  ...ListChange;
  $parent: Source;
}

@withoutOmittedProperties("data" | "list")
model AutoChange {
  ...Change;
  $data: Source;
  $list?: AutoListChange[];
}

enum ChangeKind {
  CHANGE_KIND_UNSPECIFIED: 0,

  /** Merge the new data onto the old data */
  CHANGE_KIND_UPDATE: 1,

  /** Delete the data */
  CHANGE_KIND_DELETE: 2,

  /**
   * Force re-fetch of `service`'s `method` request, where `data` is used as a query filter
   *
   * Useful for non-normalizable data, or large/complex data that may not be needed immediately
   */
  CHANGE_KIND_INVALIDATE: 3,
}

@autoFields
model Change {
  kind?: ChangeKind;

  /** Data to change, must be normalizable */
  data: WellKnown.Any;

  /** Lists changes to apply with provided data */
  list?: ListChange[];

  /** Fully qualified service name for invalidation */
  service?: string;

  /** Method name of a `service` for invalidation */
  method?: string;
}

enum ListChangeKind {
  LIST_CHANGE_KIND_UNSPECIFIED: 0,

  /** Add the data to the end of the list */
  LIST_CHANGE_KIND_APPEND: 1,

  /** Add the data to the start of the list */
  LIST_CHANGE_KIND_PREPEND: 2,

  /** Insert the data into `index` position of the list */
  LIST_CHANGE_KIND_INSERT: 3,

  /** Remove the data from the list */
  LIST_CHANGE_KIND_REMOVE: 4,

  /** Move the data to the `index` position of the list */
  LIST_CHANGE_KIND_MOVE: 5,
}

@autoFields
model ListChange {
  kind: ListChangeKind;

  /** Message containing the list, must be normalizable */
  parent: WellKnown.Any;

  /**
   * Key of the list property in the `parent` message
   *
   * Defaults to `items` if not specified
   */
  key?: string;

  /** List index for insert/move changes */
  index?: int32;
}
