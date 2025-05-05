import "@typespec/protobuf";
import "@typespec/rest";

import "./lib.tsp";

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
  name: string;
}

model FlowChange is Resource.Change<Flow>;
model FlowListItem is Resource.List.Item<Flow>;

@autoFields
model FlowListRequest {
  ...Resource.List.Request<Flow>;
  @normalKey tagId?: Resource.Id;
}

model FlowListResponse is Resource.List.Response<{}, FlowListItem> {
  ...FlowListRequest;
}

model FlowVersionsItem is Resource.List.Item<Flow>;
model FlowVersionsRequest is KeyOf<Flow>;
model FlowVersionsResponse is Resource.List.Response<Flow, FlowVersionsItem>;
model FlowGetRequest is Resource.Get.Request<Flow>;
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

@autoFields
model FlowRunResponse {
  ...KeyOf<FlowItem.Node.Node>;
  state: FlowItem.Node.NodeState;
  ...API.Change.Changes;
}

@Protobuf.service
interface FlowService {
  FlowList(...FlowListRequest): FlowListResponse;
  FlowVersions(...FlowVersionsRequest): FlowVersionsResponse;
  FlowGet(...FlowGetRequest): FlowGetResponse;
  FlowCreate(...FlowCreateRequest): FlowCreateResponse;
  FlowUpdate(...FlowUpdateRequest): FlowUpdateResponse;
  FlowDelete(...FlowDeleteRequest): FlowDeleteResponse;

  @stream(StreamMode.Out)
  FlowRun(...FlowRunRequest): FlowRunResponse;
}
