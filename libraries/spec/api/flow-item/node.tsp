import "@typespec/protobuf";
import "@typespec/rest";

import "../../lib";

import "../assert.tsp";
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
  NODE_KIND_START: 1,
  NODE_KIND_REQUEST: 2,
  NODE_KIND_FOR: 3,
  NODE_KIND_CONDITION: 4,
}

@autoFields
model NodeBase {
  @key nodeId: Resource.Id;
  position: Position;
}

// TODO: replace with union when supported
@parentResource(Flow.Flow)
@autoFields
model Node {
  ...NodeBase;
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
  collectionId: bytes;
  endpointId: bytes;
  exampleId: bytes;
  deltaExampleId: bytes;
}

@autoFields
model NodeFor {
  ...NodeBase;
  iteration: int32;
}

@autoFields
model NodeCondition {
  ...NodeBase;
  customCondition?: string;
  simpleCondition?: SimpleCondition;
}

@autoFields
model SimpleCondition {
  conditionType: Assert.AssertKind;
  path: Reference.ReferenceKey[];
  value: string;
  type: Assert.AssertKind;
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
  nodeId: Resource.Id;
  environmentId: Resource.Id;
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
