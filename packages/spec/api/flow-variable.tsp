using DevTools;

@DevTools.project
namespace API.FlowVariable;

@DataClient.entity
@parent(Flow.Flow)
model FlowVariable {
  @key variableId: Resource.Id;
  name: string;
  value: string;
  enabled: boolean;
  description: string;
}

model FlowVariableListRequest is T.Resource.List.Request<FlowVariable>;
model FlowVariableListResponse
  is T.Resource.List.Response<TRequest = FlowVariableListRequest, TItem = FlowVariableListItem>;
model FlowVariableListItem is T.Resource.List.Item<FlowVariable>;

model FlowVariableGetRequest is T.Resource.Get.Request<FlowVariable>;
model FlowVariableGetResponse is T.Resource.Get.Response<FlowVariable>;

model FlowVariableCreateRequest is T.Resource.Create.Request<FlowVariable>;
model FlowVariableCreateResponse is T.Resource.Create.Response<FlowVariable>;

model FlowVariableUpdateRequest is T.Resource.Update.Request<FlowVariable>;
model FlowVariableUpdateResponse is T.Resource.Update.Response<FlowVariable>;

model FlowVariableDeleteRequest is T.Resource.Delete.Request<FlowVariable>;
model FlowVariableDeleteResponse is T.Resource.Delete.Response<FlowVariable>;

model FlowVariableMoveRequest {
  ...ParentKeyOf<FlowVariable>;
  ...KeyOf<FlowVariable>;
  position: Resource.MovePosition;
  targetVariableId: Resource.Id;
}

interface FlowVariableService
  extends T.Resource.CRUD<
      TResource = FlowVariable,
      // List
      TListRequest = FlowVariableListRequest,
      TListResponse = FlowVariableListResponse,
      TListItem = FlowVariableListItem,
      // Get
      TGetRequest = FlowVariableGetRequest,
      TGetResponse = FlowVariableGetResponse,
      // Create
      TCreateRequest = FlowVariableCreateRequest,
      TCreateResponse = FlowVariableCreateResponse,
      // Update
      TUpdateRequest = FlowVariableUpdateRequest,
      TUpdateResponse = FlowVariableUpdateResponse,
      // Delete
      TDeleteRequest = FlowVariableDeleteRequest,
      TDeleteResponse = FlowVariableDeleteResponse
    > {
  @DataClient.endpoint("flow.js:moveVariable")
  FlowVariableMove(...FlowVariableMoveRequest): {};
}
