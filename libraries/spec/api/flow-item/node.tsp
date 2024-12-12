import "@typespec/protobuf";
import "@typespec/rest";

import "../../lib";

import "../resource.tsp";
import "../flow.tsp";

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
  NODE_KIND_START: 1,
  NODE_KIND_REQUEST: 2,
  NODE_KIND_FOR: 3,
  NODE_KIND_CONDITION: 4,
}

@autoFields
model NodeBase {
  @key nodeId: Resource.ID;
  position: Position;
}

// TODO: replace with union when supported
@parentResource(Flow.Flow)
@autoFields
model Node {
  kind: NodeKind;
  start?: NodeStart;
  request?: NodeRequest;
  for?: NodeFor;
  condition?: NodeCondition;
}

@autoFields
model NodeStart {
  ...NodeBase;
}

@autoFields
model NodeRequest {
  ...NodeBase;
  exampleID: bytes;
}

@autoFields
model NodeFor {
  ...NodeBase;
  iteration: int32;
}

enum ConditionType {
  CONDITION_TYPE_UNSPECIFIED: 0,
  CONDITION_TYPE_EQUAL: 1,
  CONDITION_TYPE_NOT_EQUAL: 2,
  CONDITION_TYPE_CONTAINS: 3,
  CONDITION_TYPE_NOT_CONTAINS: 4,
  CONDITION_TYPE_GREATER: 5,
  CONDITION_TYPE_ASSERT_TYPE_LESS: 6,
  CONDITION_TYPE_GREATER_OR_EQUAL: 7,
  CONDITION_TYPE_LESS_OR_EQUAL: 8,
  CONDITION_TYPE_EXISTS: 9,
  CONDITION_TYPE_NOT_EXISTS: 10,
  CONDITION_TYPE_CUSTOM: 11,
}

@autoFields
model NodeCondition {
  ...NodeBase;
  conditionType: ConditionType;
  customCondition?: string;
}

model NodeListItem is Resource.List.Item<Node>;
model NodeListRequest is Resource.List.Request<Node>;
model NodeListResponse is Resource.List.Response<NodeListItem>;
model NodeGetRequest is Resource.Get.Request<Node>;
model NodeGetResponse is Resource.Get.Response<Node>;
model NodeCreateRequest is Resource.Create.Request<Node>;
model NodeCreateResponse is Resource.Create.Response<Node>;
model NodeUpdateRequest is Resource.Update.Request<Node>;
model NodeUpdateResponse is Resource.Update.Response<Node>;
model NodeDeleteRequest is Resource.Delete.Request<Node>;
model NodeDeleteResponse is Resource.Delete.Response<Node>;

@autoFields
model NodeRunRequest {
  nodeId: Resource.ID;
  environmentId: Resource.ID;
}

enum ResultKind {
  RESULT_KIND_UNSPECIFIED: 0,
  RESULT_KIND_SUCCESS: 1,
  RESULT_KIND_FAILURE: 2,
}

@autoFields
model NodeRunResponse {
  varMap: bytes; // json map cannot be represented in typespec
  resultKind: ResultKind;
}

@Protobuf.service
interface NodeService {
  NodeList(...NodeListRequest): NodeListResponse;
  NodeGet(...NodeGetRequest): NodeGetResponse;
  NodeCreate(...NodeCreateRequest): NodeCreateResponse;
  NodeUpdate(...NodeUpdateRequest): NodeUpdateResponse;
  NodeDelete(...NodeDeleteRequest): NodeDeleteResponse;
  NodeRun(...NodeRunRequest): NodeRunResponse;
}
