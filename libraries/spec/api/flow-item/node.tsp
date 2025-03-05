import "@typespec/protobuf";
import "@typespec/rest";

import "../../lib";

import "../condition.tsp";
import "../flow.tsp";
import "../reference.tsp";
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
}

@parentResource(Flow.Flow)
@normalize
@autoFields
model Node {
  @key nodeId: Resource.Id;
  position: Position;
  name: string;
  @visibility(Resource.Mutation.None) state: NodeState;
  kind: NodeKind;
  noOp?: NodeNoOpKind;
  request?: NodeRequest;
  condition?: NodeCondition;
  for?: NodeFor;
  forEach?: NodeForEach;
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
  path: Reference.ReferenceKey[];
  condition: Condition.Condition;
  errorHandling: ErrorHandling;
}

@autoFields
model NodeCondition {
  condition: Condition.Condition;
}

model NodeChange is Resource.Change<Node>;
model NodeListItem is Resource.List.Item<Node>;
model NodeListRequest is Resource.List.Request<Node>;
model NodeListResponse is Resource.List.Response<Node, NodeListItem>;
model NodeGetRequest is Resource.Get.Request<Node>;
model NodeGetResponse is Resource.Get.Response<Node>;
model NodeCreateRequest is Resource.Create.Request<Node>;
model NodeCreateResponse is Resource.Create.Response<Node, NodeListResponse>;
model NodeUpdateRequest is Resource.Update.Request<Node>;
model NodeUpdateResponse is Resource.Update.Response<Node>;
model NodeDeleteRequest is Resource.Delete.Request<Node>;
model NodeDeleteResponse is Resource.Delete.Response<Node>;

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
}

@autoFields
model NodeRunResponse {
  state: NodeState;
}

@Protobuf.service
interface NodeService {
  NodeList(...NodeListRequest): NodeListResponse;
  NodeGet(...NodeGetRequest): NodeGetResponse;
  NodeCreate(...NodeCreateRequest): NodeCreateResponse;
  NodeUpdate(...NodeUpdateRequest): NodeUpdateResponse;
  NodeDelete(...NodeDeleteRequest): NodeDeleteResponse;

  @stream(StreamMode.Out)
  NodeRun(...NodeRunRequest): NodeRunResponse;
}
