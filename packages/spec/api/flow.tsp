using DevTools;

namespace API.Flow;

@DataClient.entity
@parent(Workspace.Workspace)
model Flow {
  @key flowId: Resource.Id;
  name: string;
  duration?: int32;
}

model FlowListRequest is T.Resource.List.Request<Flow> {
  tagId?: Resource.Id;
}
model FlowListResponse is T.Resource.List.Response<TRequest = FlowListRequest, TItem = FlowListItem>;
model FlowListItem is T.Resource.List.Item<Flow>;

model FlowGetRequest is T.Resource.Get.Request<Flow>;
model FlowGetResponse is T.Resource.Get.Response<Flow>;

model FlowCreateRequest is T.Resource.Create.Request<Flow>;
model FlowCreateResponse is T.Resource.Create.Response<Flow>;

model FlowUpdateRequest is T.Resource.Update.Request<Flow>;
model FlowUpdateResponse is T.Resource.Update.Response<Flow>;

model FlowDeleteRequest is T.Resource.Delete.Request<Flow>;
model FlowDeleteResponse is T.Resource.Delete.Response<Flow>;

model FlowVersionListRequest is KeyOf<Flow>;
model FlowVersionListResponse
  is T.Resource.List.Response<TRequest = FlowVersionListRequest, TItem = FlowVersionListItem>;
model FlowVersionListItem is T.Resource.List.Item<Flow>;

model FlowRunRequest {
  flowId: Resource.Id;
  environmentId: Resource.Id;
}

model FlowRunNodeResponse {
  ...KeyOf<Node.Node>;
  ...PickProperties<Node.Node, "state" | "info">;
}

model FlowRunExampleResponse {
  exampleId: Resource.Id;
  responseId: Resource.Id;
  versionId: Resource.Id;
}

model FlowRunResponse {
  node?: FlowRunNodeResponse;
  version?: FlowVersionListItem;
  example?: FlowRunExampleResponse;
}

interface FlowService
  extends T.Resource.CRUD<
      TResource = Flow,
      // List
      TListRequest = FlowListRequest,
      TListResponse = FlowListResponse,
      TListItem = FlowListItem,
      // Get
      TGetRequest = FlowGetRequest,
      TGetResponse = FlowGetResponse,
      // Create
      TCreateRequest = FlowCreateRequest,
      TCreateResponse = FlowCreateResponse,
      // Update
      TUpdateRequest = FlowUpdateRequest,
      TUpdateResponse = FlowUpdateResponse,
      // Delete
      TDeleteRequest = FlowDeleteRequest,
      TDeleteResponse = FlowDeleteResponse
    > {
  FlowVersionList is T.Resource.List.Operation<
    TRequest = FlowVersionListRequest,
    TResponse = FlowVersionListResponse,
    TItem = FlowVersionListItem
  >;

  @Protobuf.stream(Protobuf.StreamMode.Out)
  FlowRun(...FlowRunRequest): FlowRunResponse;
}
