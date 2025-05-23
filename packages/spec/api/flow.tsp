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
@entity
@parentResource(Workspace.Workspace)
model Flow {
  @key flowId: Resource.Id;
  name: string;
}

@autoFields
model FlowListRequest is Resource.List.Request<Flow> {
  tagId?: Resource.Id;
}

model FlowListItem is Resource.List.Item<Flow>;

model FlowVersionsItem is Resource.List.Item<Flow>;
model FlowVersionsRequest is KeyOf<Flow>;
model FlowVersionsResponse is Resource.List.Response<Flow, TRequest = FlowVersionsRequest, TItem = FlowVersionsItem>;

@autoFields
model FlowRunRequest {
  flowId: Resource.Id;
  environmentId: Resource.Id;
}

@autoFields
model FlowRunNodeResponse {
  ...KeyOf<FlowItem.Node.Node>;
  state: FlowItem.Node.NodeState;
}

@autoFields
model FlowRunExampleResponse {
  exampleId: Resource.Id;
  responseId: Resource.Id;
  versionId: Resource.Id;
}

@autoFields
model FlowRunResponse {
  node?: FlowRunNodeResponse;
  version?: FlowVersionsItem;
  example?: FlowRunExampleResponse;
}

@Protobuf.service
interface FlowService extends Resource.CRUD<Flow, TListItem = FlowListItem, TListRequest = FlowListRequest> {
  FlowVersions is Resource.List.Operation<
    Flow,
    TItem = FlowVersionsItem,
    TRequest = FlowVersionsRequest,
    TResponse = FlowVersionsResponse
  >;

  @stream(StreamMode.Out)
  FlowRun(...FlowRunRequest): FlowRunResponse;
}
