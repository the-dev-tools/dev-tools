import "@typespec/protobuf";
import "@typespec/rest";

import "../lib";

import "./workspace.tsp";
import "./resource.tsp";
import "./flow-item/node.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "flow.v1",
  options: {
    go_package: "dev-tools-spec/dist/buf/go/flow/v1;flowv1",
  },
})
namespace API.Flow;

@autoFields
@parentResource(Workspace.Workspace)
model Flow {
  @key flowID: Resource.ID;
  name: string;
}

model FlowListItem is Resource.List.Item<Flow>;
model FlowListRequest is Resource.List.Request<Flow>;
model FlowListResponse is Resource.List.Response<FlowListItem>;
model FlowGetRequest is Resource.Get.Request<Flow>;
model FlowGetResponse is Resource.Get.Response<Flow>;
model FlowCreateRequest is Resource.Create.Request<Flow>;
model FlowCreateResponse is Resource.Create.Response<Flow>;
model FlowUpdateRequest is Resource.Update.Request<Flow>;
model FlowUpdateResponse is Resource.Update.Response<Flow>;
model FlowDeleteRequest is Resource.Delete.Request<Flow>;
model FlowDeleteResponse is Resource.Delete.Response<Flow>;

@autoFields
model FlowListByTagRequest {
  flowID: Resource.ID;
  tagID: Resource.ID;
}

@autoFields
model FlowListByTagResponse is Resource.List.Response<FlowListItem>;

@autoFields
model FlowRunRequest {
  flowID: Resource.ID;
  environmentID: Resource.ID;
}

enum ResultKind {
  RESULT_KIND_UNSPECIFIED: 0,
  RESULT_KIND_ONGOING: 1,
  RESULT_KIND_SUCCESS: 2,
  RESULT_KIND_FAILURE: 3,
}

model Result {
  kind: ResultKind;
  error?: string;
}

@autoFields
model FlowRunResponse {
  currentNode: FlowItem.Node.Node;
}

@Protobuf.service
interface FlowService {
  FlowList(...FlowListRequest): FlowListResponse;
  FlowListByTag(...FlowListByTagRequest): FlowListByTagResponse;
  FlowGet(...FlowGetRequest): FlowGetResponse;
  FlowCreate(...FlowCreateRequest): FlowCreateResponse;
  FlowUpdate(...FlowUpdateRequest): FlowUpdateResponse;
  FlowDelete(...FlowDeleteRequest): FlowDeleteResponse;

  @stream(StreamMode.Out)
  FlowRun(...FlowRunRequest): FlowRunResponse;
}

