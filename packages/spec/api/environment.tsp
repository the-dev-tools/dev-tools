using DevTools;

namespace API.Environment;

@DataClient.entity
@parent(Workspace.Workspace)
model Environment {
  @key environmentId: Resource.Id;
  name: string;
  description: string;
  @visibility(T.Resource.Mutation.None) updated?: Protobuf.WellKnown.Timestamp;
  @visibility(T.Resource.Mutation.None) isGlobal: boolean;
}

model EnvironmentListRequest is T.Resource.List.Request<Environment>;
model EnvironmentListResponse
  is T.Resource.List.Response<TRequest = EnvironmentListRequest, TItem = EnvironmentListItem>;
model EnvironmentListItem is T.Resource.List.Item<Environment>;

model EnvironmentGetRequest is T.Resource.Get.Request<Environment>;
model EnvironmentGetResponse is T.Resource.Get.Response<Environment>;

model EnvironmentCreateRequest is T.Resource.Create.Request<Environment>;
model EnvironmentCreateResponse is T.Resource.Create.Response<Environment>;

model EnvironmentUpdateRequest is T.Resource.Update.Request<Environment>;
model EnvironmentUpdateResponse is T.Resource.Update.Response<Environment>;

model EnvironmentDeleteRequest is T.Resource.Delete.Request<Environment>;
model EnvironmentDeleteResponse is T.Resource.Delete.Response<Environment>;

model EnvironmentMoveRequest {
  ...ParentKeyOf<Environment>;
  ...KeyOf<Environment>;
  position: Resource.MovePosition;
  targetEnvironmentId: Resource.Id;
}

interface EnvironmentService
  extends T.Resource.CRUD<
      TResource = Environment,
      // List
      TListRequest = EnvironmentListRequest,
      TListResponse = EnvironmentListResponse,
      TListItem = EnvironmentListItem,
      // Get
      TGetRequest = EnvironmentGetRequest,
      TGetResponse = EnvironmentGetResponse,
      // Create
      TCreateRequest = EnvironmentCreateRequest,
      TCreateResponse = EnvironmentCreateResponse,
      // Update
      TUpdateRequest = EnvironmentUpdateRequest,
      TUpdateResponse = EnvironmentUpdateResponse,
      // Delete
      TDeleteRequest = EnvironmentDeleteRequest,
      TDeleteResponse = EnvironmentDeleteResponse
    > {
  @DataClient.endpoint("environment.js:move")
  EnvironmentMove(...EnvironmentMoveRequest): {};
}
