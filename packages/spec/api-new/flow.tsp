using DevTools;

namespace Api.Flow;

@TanStackDB.collection
model Flow {
  @key flowId: Id;
  name: string;
  @visibility(Lifecycle.Read) duration?: int32;
}

model FlowRunRequest {
  flowId: Id;
}

op FlowRun(...FlowRunRequest): {};

@TanStackDB.collection
model FlowVariable {
  @key flowVariableId: Id;
  flowId: Id;
  ...CommonTableFields;
}

enum Handle {
  Then,
  Else,
  Loop,
}

enum EdgeKind {
  NoOp,
}

@TanStackDB.collection
model Edge {
  @key edgeId: Id;
  flowId: Id;
  kind: EdgeKind;
  sourceId: Id;
  targetId: Id;
  sourceHandle: Handle;
}

enum NodeKind {
  NoOp,
  Request,
  Condition,
  For,
  ForEach,
  Js,
}

enum NodeState {
  Running,
  Success,
  Failure,
  Canceled,
}

model Position {
  x: float32;
  y: float32;
}

@TanStackDB.collection
model Node {
  @key nodeId: Id;
  flowId: Id;
  kind: NodeKind;
  name: string;
  position: Position;
  @visibility(Lifecycle.Read) state: NodeState;
  @visibility(Lifecycle.Read) info?: string;
}

enum NodeNoOpKind {
  Start,
  Create,
  Then,
  Else,
  Loop,
}

@TanStackDB.collection
model NodeNoOp {
  @key nodeId: Id;
  kind: NodeNoOpKind;
}

@TanStackDB.collection
model NodeHttp {
  @key nodeId: Id;
  httpId: Id;
}

enum ErrorHandling {
  Ignore,
  Break,
}

@TanStackDB.collection
model NodeFor {
  @key nodeId: Id;
  iterations: int32;
  condition: string;
  errorHandling: ErrorHandling;
}

@TanStackDB.collection
model NodeForEach {
  @key nodeId: Id;
  path: string;
  condition: string;
  errorHandling: ErrorHandling;
}

@TanStackDB.collection
model NodeCondition {
  @key nodeId: Id;
  condition: string;
}

@TanStackDB.collection
model NodeJs {
  @key nodeId: Id;
  code: string;
}

// TODO: read only
@TanStackDB.collection
model NodeExecution {
  @key nodeExecutionId: Id;
  nodeId: Id;
  name: string;
  state: NodeState;
  error?: string;
  input?: Protobuf.WellKnown.Json;
  output?: Protobuf.WellKnown.Json;
  httpResponseId?: Id;
  completedAt?: Protobuf.WellKnown.Timestamp;
}
