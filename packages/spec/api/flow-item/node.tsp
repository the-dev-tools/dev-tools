using DevTools;

namespace API.FlowItem.Node;

model Position {
  x: float32;
  y: float32;
}

enum NodeKind {
  NoOp,
  Request,
  Condition,
  For,
  ForEach,
  Js,
}

@parent(Flow.Flow)
@DataClient.entity
model Node {
  @key nodeId: Resource.Id;
  kind: NodeKind;
  @visibility(Resource.Query.List) position: Position;
  @visibility(Resource.Query.List) noOp?: NodeNoOpKind;
  @visibility(Resource.Query.List, Resource.Mutation.None) state: NodeState;
  @visibility(Resource.Query.List, Resource.Mutation.None) info?: string;
  @visibility(Resource.Query.Get) name: string;
  @visibility(Resource.Query.Get) request?: NodeRequest;
  @visibility(Resource.Query.Get) condition?: NodeCondition;
  @visibility(Resource.Query.Get) for?: NodeFor;
  @visibility(Resource.Query.Get) forEach?: NodeForEach;
  @visibility(Resource.Query.Get) js?: NodeJS;
}

enum NodeNoOpKind {
  Start,
  Create,
  Then,
  Else,
  Loop,
}

model NodeRequest {
  collectionId: bytes;
  endpointId: bytes;
  exampleId: bytes;
  deltaEndpointId: bytes;
  deltaExampleId: bytes;
}

enum ErrorHandling {
  Ignore,
  Break,
}

model NodeFor {
  iterations: int32;
  condition: Condition.Condition;
  errorHandling: ErrorHandling;
}

model NodeForEach {
  path: string;
  condition: Condition.Condition;
  errorHandling: ErrorHandling;
}

model NodeCondition {
  condition: Condition.Condition;
}

model NodeJS {
  code: string;
}

model NodeRunRequest {
  nodeId: Resource.Id;
  environmentId: Resource.Id;
}

enum NodeState {
  Running,
  Success,
  Failure,
  Canceled,
}

model NodeRunResponse {
  state: NodeState;
}

interface NodeService extends Resource.CRUD<Node> {
  @Protobuf.stream(Protobuf.StreamMode.Out)
  NodeRun(...NodeRunRequest): NodeRunResponse;
}
