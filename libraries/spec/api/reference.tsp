import "@typespec/protobuf";

import "../lib";

import "./resource.tsp";

using TypeSpec.Protobuf;

@package({
  name: "reference.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/reference/v1;referencev1",
  },
})
namespace API.Reference;

enum ReferenceKeyKind {
  REFERENCE_KEY_KIND_UNSPECIFIED: 0,
  REFERENCE_KEY_KIND_GROUP: 1,
  REFERENCE_KEY_KIND_KEY: 2,
  REFERENCE_KEY_KIND_INDEX: 3,
  REFERENCE_KEY_KIND_ANY: 4,
}

@autoFields
model ReferenceKey {
  kind: ReferenceKeyKind;
  group?: string;
  key?: string;
  index?: int32;
  any?: WellKnown.Empty;
}

enum ReferenceKind {
  REFERENCE_KIND_UNSPECIFIED: 0,
  REFERENCE_KIND_MAP: 1,
  REFERENCE_KIND_ARRAY: 2,
  REFERENCE_KIND_VALUE: 3,
  REFERENCE_KIND_VARIABLE: 4,
}

@autoFields
model Reference {
  kind: ReferenceKind;
  key: ReferenceKey;

  /** Child map references */
  map?: Reference[];

  /** Child array references */
  array?: Reference[];

  /** Primitive value as JSON string */
  value?: string;

  /** Environment names containing the variable */
  variable?: string[];
}

@autoFields
model ReferenceGetRequest {
  workspaceId?: Resource.Id;
  exampleId?: Resource.Id;
  nodeId?: Resource.Id;
}

@autoFields
model ReferenceGetResponse {
  items: Reference[];
}

@Protobuf.service
interface ReferenceService {
  ReferenceGet(...ReferenceGetRequest): ReferenceGetResponse;
}
