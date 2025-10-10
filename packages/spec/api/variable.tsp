using DevTools;

namespace API.Variable;

@DataClient.entity
@parent(Environment.Environment)
model Variable {
  @key variableId: Resource.Id;
  name: string;
  value: string;
  enabled: boolean;
  description: string;
}

model VariableListRequest is T.Resource.List.Request<Variable> {
  @normalKey workspaceId?: Resource.Id;
}
model VariableListResponse is T.Resource.List.Response<TRequest = VariableListRequest, TItem = VariableListItem>;
model VariableListItem is T.Resource.List.Item<Variable>;

model VariableGetRequest is T.Resource.Get.Request<Variable>;
model VariableGetResponse is T.Resource.Get.Response<Variable>;

model VariableCreateRequest is T.Resource.Create.Request<Variable> {
  @normalKey workspaceId?: Resource.Id;
}
model VariableCreateResponse is T.Resource.Create.Response<Variable>;

model VariableUpdateRequest is T.Resource.Update.Request<Variable>;
model VariableUpdateResponse is T.Resource.Update.Response<Variable>;

model VariableDeleteRequest is T.Resource.Delete.Request<Variable>;
model VariableDeleteResponse is T.Resource.Delete.Response<Variable>;

model VariableMoveRequest {
  ...ParentKeyOf<Variable>;
  ...KeyOf<Variable>;
  position: Resource.MovePosition;
  targetVariableId: Resource.Id;
}

interface VariableService
  extends T.Resource.CRUD<
      TResource = Variable,
      // List
      TListRequest = VariableListRequest,
      TListResponse = VariableListResponse,
      TListItem = VariableListItem,
      // Get
      TGetRequest = VariableGetRequest,
      TGetResponse = VariableGetResponse,
      // Create
      TCreateRequest = VariableCreateRequest,
      TCreateResponse = VariableCreateResponse,
      // Update
      TUpdateRequest = VariableUpdateRequest,
      TUpdateResponse = VariableUpdateResponse,
      // Delete
      TDeleteRequest = VariableDeleteRequest,
      TDeleteResponse = VariableDeleteResponse
    > {
  @DataClient.endpoint("variable.js:move")
  VariableMove(...VariableMoveRequest): {};
}
