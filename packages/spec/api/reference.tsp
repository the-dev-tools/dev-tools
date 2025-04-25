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
model ReferenceTreeItem {
  kind: ReferenceKind;
  key: ReferenceKey;

  /** Child map references */
  map?: ReferenceTreeItem[];

  /** Child array references */
  array?: ReferenceTreeItem[];

  /** Primitive value as JSON string */
  value?: string;

  /** Environment names containing the variable */
  variable?: string[];
}

@autoFields
model ReferenceContext {
  workspaceId?: Resource.Id;
  exampleId?: Resource.Id;
  nodeId?: Resource.Id;
}

model ReferenceTreeRequest is ReferenceContext;

@autoFields
model ReferenceTreeResponse {
  items: ReferenceTreeItem[];
}

@autoFields
model ReferenceCompletion {
  kind: ReferenceKind;

  /** End token of the string to be completed, i.e. 'body' in 'response.bo|dy' */
  endToken: string;

  /** Index of the completion start in the end token, i.e. 2 in 'bo|dy' of 'response.bo|dy' */
  endIndex: int32;

  /** Number of items when reference is a map or an array */
  itemCount?: int32;

  /** Environment names when reference is a variable */
  environments?: string[];
}

@autoFields
model ReferenceCompletionRequest is ReferenceContext {
  /** Start of the string to be completed, i.e. 'response.bo' in 'response.bo|dy' */
  start: string;
}

@autoFields
model ReferenceCompletionResponse {
  items: ReferenceCompletion[];
}

@autoFields
model ReferenceValueRequest is ReferenceContext {
  path: string;
}

@autoFields
model ReferenceValueResponse {
  value: string;
}

@Protobuf.service
interface ReferenceService {
  ReferenceTree(...ReferenceTreeRequest): ReferenceTreeResponse;
  ReferenceCompletion(...ReferenceCompletionRequest): ReferenceCompletionResponse;
  ReferenceValue(...ReferenceValueRequest): ReferenceValueResponse;
}
