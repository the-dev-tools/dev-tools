import "@typespec/protobuf";
import "@typespec/rest";

import "../lib.tsp";

import "../condition.tsp";
import "../flow.tsp";
import "../resource.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "flow.node.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/flow/node/v1;nodev1",
  },
})
namespace API.FlowItem.Node;

@autoFields
model Position {
  x: float32;
  y: float32;
}

enum NodeKind {
  NODE_KIND_UNSPECIFIED: 0,
  NODE_KIND_NO_OP: 1,
  NODE_KIND_REQUEST: 2,
  NODE_KIND_CONDITION: 3,
  NODE_KIND_FOR: 4,
  NODE_KIND_FOR_EACH: 5,
  NODE_KIND_JS: 6,
}

@parentResource(Flow.Flow)
@normalize
@entity
@autoFields
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
  NODE_NO_OP_KIND_UNSPECIFIED: 0,
  NODE_NO_OP_KIND_START: 1,
  NODE_NO_OP_KIND_CREATE: 2,
  NODE_NO_OP_KIND_THEN: 3,
  NODE_NO_OP_KIND_ELSE: 4,
  NODE_NO_OP_KIND_LOOP: 5,
}

@autoFields
model NodeRequest {
  collectionId: bytes;
  endpointId: bytes;
  exampleId: bytes;
  deltaEndpointId: bytes;
  deltaExampleId: bytes;
}

enum ErrorHandling {
  ERROR_HANDLING_UNSPECIFIED: 0,
  ERROR_HANDLING_IGNORE: 1,
  ERROR_HANDLING_BREAK: 2,
}

@autoFields
model NodeFor {
  iterations: int32;
  condition: Condition.Condition;
  errorHandling: ErrorHandling;
}

@autoFields
model NodeForEach {
  path: string;
  condition: Condition.Condition;
  errorHandling: ErrorHandling;
}

@autoFields
model NodeCondition {
  condition: Condition.Condition;
}

@autoFields
model NodeJS {
  code: string;
}

@autoFields
model NodeRunRequest {
  nodeId: Resource.Id;
  environmentId: Resource.Id;
}

enum NodeState {
  NODE_STATE_UNSPECIFIED: 0,
  NODE_STATE_RUNNING: 1,
  NODE_STATE_SUCCESS: 2,
  NODE_STATE_FAILURE: 3,
  NODE_STATE_CANCELED: 4,
}

@autoFields
model NodeRunResponse {
  state: NodeState;
}

@Protobuf.service
interface NodeService extends Resource.CRUD<Node> {
  @stream(StreamMode.Out)
  NodeRun(...NodeRunRequest): NodeRunResponse;
}
