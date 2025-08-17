using DevTools;

namespace API.Flow;

@DataClient.entity
@parent(Workspace.Workspace)
model Flow {
  @key flowId: Resource.Id;
  name: string;
}

model FlowListRequest is Resource.List.ListRequest<Flow> {
  tagId?: Resource.Id;
}

model FlowVersion is Flow;

@instanceOf(Resource.List.ListRequest<FlowVersion>)
model FlowVersionListRequest is KeyOf<FlowVersion>;

model FlowRunRequest {
  flowId: Resource.Id;
  environmentId: Resource.Id;
}

model FlowRunNodeResponse {
  ...KeyOf<FlowItem.Node.Node>;
  ...PickProperties<FlowItem.Node.Node, "state" | "info">;
}

model FlowRunExampleResponse {
  exampleId: Resource.Id;
  responseId: Resource.Id;
  versionId: Resource.Id;
}

model FlowRunResponse {
  node?: FlowRunNodeResponse;
  version?: FlowVersion;
  example?: FlowRunExampleResponse;
}

interface FlowService extends Resource.CRUD<Flow>, Resource.List.Interface<FlowVersion> {
  @Protobuf.stream(Protobuf.StreamMode.Out)
  FlowRun(...FlowRunRequest): FlowRunResponse;
}
