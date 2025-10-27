using DevTools;

namespace Api.Reference;

enum ReferenceKeyKind {
  Group,
  Key,
  Index,
  Any,
}

model ReferenceKey {
  kind: ReferenceKeyKind;
  group?: string;
  key?: string;
  index?: int32;
  any?: Protobuf.WellKnown.Empty;
}

enum ReferenceKind {
  Map,
  Array,
  Value,
  Variable,
}

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

model ReferenceContext {
  workspaceId?: Id;
  exampleId?: Id;
  nodeId?: Id;
}

model ReferenceTreeRequest is ReferenceContext;

model ReferenceTreeResponse {
  items: ReferenceTreeItem[];
}

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

model ReferenceCompletionRequest is ReferenceContext {
  /** Start of the string to be completed, i.e. 'response.bo' in 'response.bo|dy' */
  start: string;
}

model ReferenceCompletionResponse {
  items: ReferenceCompletion[];
}

model ReferenceValueRequest is ReferenceContext {
  path: string;
}

model ReferenceValueResponse {
  value: string;
}

interface ReferenceService {
  ReferenceTree(...ReferenceTreeRequest): ReferenceTreeResponse;
  ReferenceCompletion(...ReferenceCompletionRequest): ReferenceCompletionResponse;
  ReferenceValue(...ReferenceValueRequest): ReferenceValueResponse;
}
