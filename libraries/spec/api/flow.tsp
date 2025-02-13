import "@typespec/protobuf";
import "@typespec/rest";

import "../lib";

import "./workspace.tsp";
import "./resource.tsp";
import "./flow-item";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "flow.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/flow/v1;flowv1",
  },
})
namespace API.Flow;

@autoFields
@normalize
@parentResource(Workspace.Workspace)
model Flow {
  @key flowId: Resource.Id;
  @visibility("query") @normalKey flowVersionId?: Resource.Id;
  name: string;
}

model FlowChange is Resource.Change<Flow>;
model FlowListItem is Resource.List.Item<Flow>;

@autoFields
model FlowListRequest {
  ...Resource.List.Request<Flow>;
  @normalKey flowParentId?: Resource.Id;
  @normalKey tagId?: Resource.Id;
}

model FlowListResponse is Resource.List.Response<{}, FlowListItem> {
  ...FlowListRequest;
}

model FlowGetRequest is Resource.Get.Request<Flow> {
  @autoField @normalKey flowVersionId?: Resource.Id;
}
model FlowGetResponse is Resource.Get.Response<Flow>;
model FlowCreateRequest is Resource.Create.Request<Flow>;
model FlowCreateResponse is Resource.Create.Response<Flow, FlowListResponse>;
model FlowUpdateRequest is Resource.Update.Request<Flow>;
model FlowUpdateResponse is Resource.Update.Response<Flow>;
model FlowDeleteRequest is Resource.Delete.Request<Flow>;
model FlowDeleteResponse is Resource.Delete.Response<Flow>;

@autoFields
model FlowRunRequest {
  flowId: Resource.Id;
  environmentId: Resource.Id;
}

enum ResultKind {
  RESULT_KIND_UNSPECIFIED: 0,
  RESULT_KIND_SUCCESS: 1,
  RESULT_KIND_FAILURE: 2,
  RESULT_KIND_ONGOING: 3,
}

model Result {
  kind: ResultKind;
  error?: string;
}

enum NodeState {
  NODE_STATE_UNSPECIFIED: 0,
  NODE_STATE_RUNNING: 1,
  NODE_STATE_SUCCESS: 2,
  NODE_STATE_FAILURE: 3,
}

@autoFields
model FlowRunResponse {
  ...KeyOf<FlowItem.Node.Node>;
  state: NodeState;
}

@Protobuf.service
interface FlowService {
  FlowList(...FlowListRequest): FlowListResponse;
  FlowGet(...FlowGetRequest): FlowGetResponse;
  FlowCreate(...FlowCreateRequest): FlowCreateResponse;
  FlowUpdate(...FlowUpdateRequest): FlowUpdateResponse;
  FlowDelete(...FlowDeleteRequest): FlowDeleteResponse;

  @stream(StreamMode.Out)
  FlowRun(...FlowRunRequest): FlowRunResponse;
}
