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
  key: string;
  map?: Map<string, Reference>;
  array?: Reference[];

  /** Primitive value - string, number, boolean, or null */
  value?: WellKnown.Any;

  /** Environment names with the variable */
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
  root: Reference;
}

@Protobuf.service
interface ReferenceService {
  ReferenceGet(...ReferenceGetRequest): ReferenceGetResponse;
}
